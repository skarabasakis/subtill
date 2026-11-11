package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	sqlite3 "modernc.org/sqlite/lib"
)

const backupTimeLayout = "20060102T150405"

// migrateSchema runs the migration procedure on the outdated database at path.
// If a migration fails, the database is restored from the backup taken first.
func (s *Service) migrateSchema(path string) error {
	err := s.withMigrationPool(path, func(pool *sql.DB) error {
		return s.migrateLockedDatabase(pool, path)
	})
	if backup := failedMigrationBackup(err); backup != "" {
		return errors.Join(err, restore(path, backup))
	}
	return err
}

func (s *Service) withMigrationPool(path string, fn func(*sql.DB) error) error {
	return withSingleConnection(s.migrationDSN(path), fn)
}

// migrateLockedDatabase locks the database behind pool, backs it up and
// migrates it if it is still outdated.
func (s *Service) migrateLockedDatabase(pool *sql.DB, path string) error {
	if err := lockExclusively(pool); err != nil {
		return err
	}

	// The probe has confirmed schema_migrations exists, so building the
	// instance doesn't create it.
	m, err := s.newMigrate(pool)
	if err != nil {
		return err
	}

	// Another process may have migrated the file since it was probed.
	state, err := migratorSchemaState(m)
	if err != nil {
		return err
	}
	if err := s.checkSchemaState(path, state); !errors.Is(err, ErrSchemaOutdated) {
		return err
	}

	backup := backupPath(path, state.version, time.Now())
	if err := backUpDatabase(pool, backup); err != nil {
		return err
	}
	if err := runMigrations(m, pool); err != nil {
		failed := &SchemaError{Err: ErrMigrationFailed, Found: state.version, Expected: s.schemaVersion, Backup: backup}
		return fmt.Errorf("%w: %w", failed, err)
	}
	return nil
}

func failedMigrationBackup(err error) string {
	var se *SchemaError
	if errors.As(err, &se) && errors.Is(se.Err, ErrMigrationFailed) {
		return se.Backup
	}
	return ""
}

// lockExclusively takes an exclusive lock that the connection keeps until it
// closes, because of locking_mode(EXCLUSIVE). Every other open connection
// holds a shared lock in WAL mode, so this fails while another process uses
// the file.
func lockExclusively(db *sql.DB) error {
	if _, err := db.Exec("BEGIN EXCLUSIVE; COMMIT;"); err != nil {
		if sqliteCode(err) == sqlite3.SQLITE_BUSY {
			return fmt.Errorf("%w: %w", ErrDatabaseInUse, err)
		}
		return fmt.Errorf("lock database: %w", err)
	}
	return nil
}

// newMigrate returns a golang-migrate instance for db, creating
// schema_migrations if it doesn't exist. Don't close the instance: its Close
// closes db, which the caller owns.
func (s *Service) newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}
	src, err := iofs.New(s.migrations, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return nil, fmt.Errorf("migration instance: %w", err)
	}
	return m, nil
}

func migratorSchemaState(m *migrate.Migrate) (schemaState, error) {
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return schemaState{}, fmt.Errorf("read schema version: %w", err)
	}
	return schemaState{version: version, dirty: dirty}, nil
}

func backUpDatabase(db *sql.DB, backup string) error {
	if _, err := db.Exec("VACUUM INTO ?", backup); err != nil {
		return fmt.Errorf("back up database: %w", err)
	}
	return nil
}

// runMigrations applies all pending migrations and checks foreign keys.
func runMigrations(m *migrate.Migrate, db *sql.DB) error {
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return checkForeignKeys(db)
}

func checkForeignKeys(db *sql.DB) error {
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("foreign key check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("foreign key check: %w", err)
		}
		return fmt.Errorf("foreign key violation: row %d of table %s references a missing row of %s",
			rowid.Int64, table, parent)
	}
	return rows.Err()
}

func backupPath(path string, version uint, now time.Time) string {
	return fmt.Sprintf("%s.%s.v%d.bak", path, now.Format(backupTimeLayout), version)
}

// newestBackup returns the path of the newest backup of the database at path,
// or "" if there is none.
func newestBackup(path string) (string, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("list backups: %w", err)
	}
	base := filepath.Base(path)
	var newest string
	var newestStamp backupStamp
	for _, entry := range entries {
		if stamp, ok := backupStampOf(base, entry); ok && stamp.isNewerThan(newestStamp) {
			newest, newestStamp = path+strings.TrimPrefix(entry.Name(), base), stamp
		}
	}
	return newest, nil
}

var backupSuffix = regexp.MustCompile(`^\.(\d{8}T\d{6})\.v(\d+)\.bak$`)

type backupStamp struct {
	time    string
	version uint64
}

// backupStampOf returns the stamp of entry if it is a backup of the database
// file named base.
func backupStampOf(base string, entry fs.DirEntry) (backupStamp, bool) {
	suffix, ok := strings.CutPrefix(entry.Name(), base)
	if !ok || entry.IsDir() {
		return backupStamp{}, false
	}
	m := backupSuffix.FindStringSubmatch(suffix)
	if m == nil {
		return backupStamp{}, false
	}
	version, err := strconv.ParseUint(m[2], 10, 64)
	if err != nil {
		return backupStamp{}, false
	}
	return backupStamp{time: m[1], version: version}, true
}

func (s backupStamp) isNewerThan(other backupStamp) bool {
	return s.time > other.time || (s.time == other.time && s.version > other.version)
}

func restore(path, backup string) error {
	if err := removeFiles(sidecarPaths(path)...); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	if err := replaceFile(path, backup); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	return nil
}

// replaceFile copies src next to dst and renames the copy over dst, so dst is
// never left partially written.
func replaceFile(dst, src string) error {
	tmp, err := copyToTempFile(src, filepath.Dir(dst), filepath.Base(dst)+".restore-*")
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// copyToTempFile copies src to a new file created by os.CreateTemp(dir,
// pattern) and returns its path. The new file is removed if the copy fails.
func copyToTempFile(src, dir, pattern string) (name string, err error) {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()

	out, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = out.Close()
			_ = os.Remove(out.Name())
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return out.Name(), nil
}

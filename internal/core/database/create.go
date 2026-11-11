package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/locale"
	"github.com/skarabasakis/subtill/internal/core/store"
)

// Create calls Service.Create on the Service with the embedded migrations.
func Create(path string, preferences domain.Preferences) error {
	return defaultService.Create(path, preferences)
}

// Create creates a new subtill database at path. Nil preferences are taken
// from the OS locale defaults. The caller validates preferences with the same
// rules as preferences.set. Create doesn't return a handle; callers open the
// new database with Open(path, false).
func (s *Service) Create(path string, preferences domain.Preferences) (err error) {
	if err := createEmptyFile(path); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, removeDatabase(path))
		}
	}()
	return s.withMigrationPool(path, func(pool *sql.DB) error {
		return s.initializeDatabase(pool, preferences)
	})
}

// createEmptyFile creates path or returns ErrAlreadyExists. O_EXCL makes the
// existence check and the creation one step. A zero-byte file is a valid
// empty SQLite database.
func createEmptyFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- path is chosen by the user
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrAlreadyExists
		}
		return err
	}
	if err := f.Close(); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func (s *Service) initializeDatabase(pool *sql.DB, preferences domain.Preferences) error {
	m, err := s.newMigrate(pool)
	if err != nil {
		return err
	}
	if err := runMigrations(m, pool); err != nil {
		return fmt.Errorf("%w: %w", &SchemaError{Err: ErrMigrationFailed, Expected: s.schemaVersion}, err)
	}
	if err := setApplicationID(pool); err != nil {
		return err
	}
	return storePreferences(pool, preferences.AppliedTo(locale.DefaultPreferences()))
}

func setApplicationID(db *sql.DB) error {
	if _, err := db.Exec(fmt.Sprintf("PRAGMA application_id = %d", SubtillAppID)); err != nil {
		return fmt.Errorf("set application id: %w", err)
	}
	return nil
}

func storePreferences(db *sql.DB, record domain.PreferencesRecord) error {
	if err := (store.PreferencesRepository{}).Upsert(sqlx.NewDb(db, driverName), record); err != nil {
		return fmt.Errorf("store preferences: %w", err)
	}
	return nil
}

func removeDatabase(path string) error {
	return removeFiles(append([]string{path}, sidecarPaths(path)...)...)
}

func sidecarPaths(path string) []string {
	return []string{path + "-wal", path + "-shm"}
}

// removeFiles removes every path that exists and joins the errors.
func removeFiles(paths ...string) error {
	var errs []error
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

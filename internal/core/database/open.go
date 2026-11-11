package database

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	sqlite3 "modernc.org/sqlite/lib"
)

type schemaState struct {
	version uint
	dirty   bool
}

// Open calls Service.Open on the Service with the embedded migrations.
func Open(path string, migrate bool) (db *sqlx.DB, closeFn func() error, err error) {
	return defaultService.Open(path, migrate)
}

// Open opens the subtill database at path and returns the main pool and the
// function that closes it. An outdated schema is migrated only when migrate is
// true; otherwise Open returns ErrSchemaOutdated. On error, db and closeFn are
// nil and every connection Open opened is closed.
//
// Close the pool with closeFn, not db.Close, which skips the WAL checkpoint.
func (s *Service) Open(path string, migrate bool) (db *sqlx.DB, closeFn func() error, err error) {
	err = s.checkDatabase(path)
	if migrate && errors.Is(err, ErrSchemaOutdated) {
		err = s.migrateSchema(path)
	}
	if err != nil {
		return nil, nil, err
	}
	return s.openMainPool(path)
}

// checkDatabase returns an error unless path is a subtill database with the
// current schema. The probe connection is closed when it returns.
func (s *Service) checkDatabase(path string) error {
	return s.withProbe(path, func(probe *sql.DB) error {
		if err := checkOpenable(probe, path); err != nil {
			return err
		}
		if err := checkSubtillAppID(probe); err != nil {
			return err
		}
		if err := checkMigrationsTable(probe); err != nil {
			return err
		}
		state, err := storedSchemaState(probe)
		if err != nil {
			return err
		}
		return s.checkSchemaState(path, state)
	})
}

func (s *Service) withProbe(path string, fn func(*sql.DB) error) error {
	return withSingleConnection(s.probeDSN(path), fn)
}

// withSingleConnection runs fn on a pool of one connection to dsn, then
// closes the pool.
func withSingleConnection(dsn string, fn func(*sql.DB) error) error {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	err = fn(db)
	if closeErr := db.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return err
}

func checkOpenable(db *sql.DB, path string) error {
	err := db.Ping()
	if err == nil {
		return nil
	}
	switch sqliteCode(err) {
	case sqlite3.SQLITE_CANTOPEN:
		return fmt.Errorf("%w: %s", ErrNotFound, path)
	case sqlite3.SQLITE_NOTADB:
		return ErrNotSubtillDatabase
	}
	return fmt.Errorf("open database: %w", err)
}

// checkSubtillAppID returns ErrNotSubtillDatabase if the application_id of
// db isn't SubtillAppID.
func checkSubtillAppID(db *sql.DB) error {
	var appID int64
	if err := db.QueryRow("PRAGMA application_id").Scan(&appID); err != nil {
		return fmt.Errorf("read application id: %w", err)
	}
	if appID != SubtillAppID {
		return ErrNotSubtillDatabase
	}
	return nil
}

// checkMigrationsTable returns ErrNoSchemaMigrationsTable if db has no
// golang-migrate schema_migrations table.
func checkMigrationsTable(db *sql.DB) error {
	var tables int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`,
	).Scan(&tables)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if tables == 0 {
		return ErrNoSchemaMigrationsTable
	}
	return nil
}

// storedSchemaState reads schema_migrations directly: golang-migrate's driver
// would create the table if it were missing and reports read errors as
// "no version".
func storedSchemaState(db *sql.DB) (schemaState, error) {
	var version int64
	var dirty bool
	err := db.QueryRow(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return schemaState{}, fmt.Errorf("read schema version: %w", err)
	}
	return schemaState{version: uint(max(version, 0)), dirty: dirty}, nil
}

// checkSchemaState returns a *SchemaError if state isn't SchemaVersion:
// ErrSchemaDirty, ErrSchemaTooNew, or ErrSchemaOutdated when migrations are
// pending.
func (s *Service) checkSchemaState(path string, state schemaState) error {
	switch {
	case state.dirty:
		backup, err := newestBackup(path)
		if err != nil {
			return err
		}
		return &SchemaError{Err: ErrSchemaDirty, Found: state.version, Expected: s.schemaVersion, Backup: backup}
	case state.version > s.schemaVersion:
		return &SchemaError{Err: ErrSchemaTooNew, Found: state.version, Expected: s.schemaVersion}
	case state.version < s.schemaVersion:
		return &SchemaError{Err: ErrSchemaOutdated, Found: state.version, Expected: s.schemaVersion}
	}
	return nil
}

func (s *Service) openMainPool(path string) (*sqlx.DB, func() error, error) {
	db, err := sqlx.Open(driverName, s.mainDSN(path))
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, nil, errors.Join(fmt.Errorf("open database: %w", err), db.Close())
	}
	return db, checkpointingCloseFunc(db), nil
}

// checkpointingCloseFunc checkpoints the WAL into the database file before
// closing, so the file alone holds every commit of this process even while
// another process keeps the database open.
func checkpointingCloseFunc(db *sqlx.DB) func() error {
	var once sync.Once
	return func() (err error) {
		once.Do(func() {
			_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
			err = db.Close()
		})
		return err
	}
}

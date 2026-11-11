package database

import (
	"errors"
	"fmt"

	"modernc.org/sqlite"
)

var (
	ErrNotFound                = errors.New("database file not found or inaccessible")
	ErrAlreadyExists           = errors.New("database file already exists")
	ErrNotSubtillDatabase      = errors.New("not a subtill database")
	ErrNoSchemaMigrationsTable = errors.New("subtill database has no schema_migrations table")
	ErrSchemaOutdated          = errors.New("database schema is outdated")
	ErrSchemaTooNew            = errors.New("database schema is newer than this version of subtill supports")
	ErrSchemaDirty             = errors.New("a previous database migration was interrupted")
	ErrDatabaseInUse           = errors.New("database is in use by another process")
	ErrMigrationFailed         = errors.New("database migration failed")
)

// SchemaError carries the details of ErrSchemaOutdated, ErrSchemaTooNew,
// ErrSchemaDirty and ErrMigrationFailed.
type SchemaError struct {
	Err      error  // ErrSchemaOutdated, ErrSchemaTooNew, ErrSchemaDirty or ErrMigrationFailed
	Found    uint   // schema version of the file
	Expected uint   // SchemaVersion
	Backup   string // backup path, if one exists
}

func (e *SchemaError) Error() string {
	msg := fmt.Sprintf("%v (found schema version %d, expected %d)", e.Err, e.Found, e.Expected)
	if e.Backup != "" {
		msg += ", backup: " + e.Backup
	}
	return msg
}

func (e *SchemaError) Unwrap() error { return e.Err }

// sqliteCode returns the primary SQLite result code of err, or 0.
func sqliteCode(err error) int {
	var e *sqlite.Error
	if errors.As(err, &e) {
		return e.Code() & 0xff
	}
	return 0
}

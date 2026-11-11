// Package database creates, opens, migrates and closes subtill database files.
// It is the only code that opens SQLite connections; every other component
// receives the *sqlx.DB returned by Open.
//
// See AGENTS.md in this directory.
package database

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/skarabasakis/subtill/internal/core/database/migrations"
)

// SubtillAppID is the SQLite header application_id of every subtill database
// (ASCII "SUBT").
const SubtillAppID = 0x53554254

const defaultBusyTimeout = 5 * time.Second

var defaultService = mustNew(migrations.FS)

// SchemaVersion is the version of the newest embedded migration.
var SchemaVersion = defaultService.SchemaVersion()

// Service creates and opens subtill databases with a given migration source.
// The package-level functions use a Service with the embedded migrations; use
// New only when a different migration source is needed.
type Service struct {
	migrations    fs.FS
	busyTimeout   time.Duration
	schemaVersion uint // derived from migrations
}

type Option func(*Service)

// WithBusyTimeout sets how long connections wait for a lock held by another
// connection. The default is 5 seconds.
func WithBusyTimeout(d time.Duration) Option {
	return func(s *Service) { s.busyTimeout = d }
}

// New returns a Service that migrates databases with the golang-migrate
// migrations in the root of migrations.
func New(migrations fs.FS, opts ...Option) (*Service, error) {
	s := &Service{migrations: migrations, busyTimeout: defaultBusyTimeout}
	for _, opt := range opts {
		opt(s)
	}
	version, err := latestVersion(migrations)
	if err != nil {
		return nil, err
	}
	s.schemaVersion = version
	return s, nil
}

func mustNew(migrations fs.FS, opts ...Option) *Service {
	s, err := New(migrations, opts...)
	if err != nil {
		panic(fmt.Sprintf("database: embedded migrations: %v", err))
	}
	return s
}

// SchemaVersion is the version of the newest migration of s.
func (s *Service) SchemaVersion() uint {
	return s.schemaVersion
}

// latestVersion walks the migration source with First and Next, the same way
// golang-migrate does, and returns the last version.
func latestVersion(migrations fs.FS) (uint, error) {
	src, err := iofs.New(migrations, ".")
	if err != nil {
		return 0, err
	}
	defer func() { _ = src.Close() }()
	version, err := src.First()
	if err != nil {
		return 0, fmt.Errorf("first migration: %w", err)
	}
	for {
		next, err := src.Next(version)
		if errors.Is(err, fs.ErrNotExist) {
			return version, nil
		}
		if err != nil {
			return 0, fmt.Errorf("migration after %d: %w", version, err)
		}
		version = next
	}
}

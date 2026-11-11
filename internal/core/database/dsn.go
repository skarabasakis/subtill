package database

import (
	"fmt"
	"path/filepath"
	"strings"
)

const driverName = "sqlite"

// probeDSN opens the file for the identity and version checks. It sets no
// pragma that changes the file. It is read-write only so that, as the last
// connection to a WAL database, closing it removes the -wal and -shm files
// it had to create; a read-only connection leaves them behind.
func (s *Service) probeDSN(path string) string {
	return fmt.Sprintf("%s?mode=rw&_pragma=busy_timeout(%d)", fileURI(path), s.busyTimeout.Milliseconds())
}

func (s *Service) mainDSN(path string) string {
	return fmt.Sprintf(
		"%s?mode=rw&_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_txlock=immediate",
		fileURI(path), s.busyTimeout.Milliseconds())
}

// migrationDSN leaves foreign keys off: migration files run in transactions,
// where PRAGMA foreign_keys is a no-op, and a table rebuild must not cascade
// deletes.
func (s *Service) migrationDSN(path string) string {
	return fmt.Sprintf(
		"%s?mode=rw&_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=locking_mode(EXCLUSIVE)&_txlock=immediate",
		fileURI(path), s.busyTimeout.Milliseconds())
}

var uriPathEscaper = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

// fileURI returns a SQLite URI filename for path. URI parameters such as mode
// are only honored in URI form.
func fileURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows drive letter: file:/C:/...
	}
	return "file:" + uriPathEscaper.Replace(p)
}

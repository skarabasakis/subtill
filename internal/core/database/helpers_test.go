package database_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var uriPathEscaper = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

// fileURI returns a SQLite URI filename for path, so that mode parameters are
// honored.
func fileURI(t testing.TB, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file:" + uriPathEscaper.Replace(p)
}

// rawDB opens or creates path without going through the database package,
// with a single connection and no pragmas besides a busy timeout. The
// connection is closed at the end of the test; close it earlier before calling
// the database package.
func rawDB(t testing.TB, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", fileURI(t, path)+"?mode=rwc&_pragma=busy_timeout(5000)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// rawExec runs statements on path through rawDB and closes the connection.
func rawExec(t testing.TB, path string, statements ...string) {
	t.Helper()
	db := rawDB(t, path)
	for _, s := range statements {
		_, err := db.Exec(s)
		require.NoError(t, err, s)
	}
	require.NoError(t, db.Close())
}

// rawQueryInt returns the single integer result of query on path.
func rawQueryInt(t testing.TB, path, query string) int64 {
	t.Helper()
	db := rawDB(t, path)
	var v int64
	require.NoError(t, db.QueryRow(query).Scan(&v), query)
	require.NoError(t, db.Close())
	return v
}

// rawSchemaState returns the version and dirty flag stored in path.
func rawSchemaState(t testing.TB, path string) (uint, bool) {
	t.Helper()
	db := rawDB(t, path)
	var version uint
	var dirty bool
	require.NoError(t, db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.NoError(t, db.Close())
	return version, dirty
}

func digest(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertNoSidecars(t testing.TB, path string) {
	t.Helper()
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		_, err := os.Stat(sidecar)
		assert.ErrorIs(t, err, fs.ErrNotExist, "%s exists", filepath.Base(sidecar))
	}
}

// assertNoOpenConnections asserts that an exclusive lock on path is available
// immediately. In WAL mode every open connection holds a shared lock, so this
// fails while any connection to the file is open. Files that aren't SQLite
// databases can't be locked by SQLite connections, so they pass.
func assertNoOpenConnections(t testing.TB, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", fileURI(t, path)+"?mode=rw&_pragma=locking_mode(EXCLUSIVE)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer func() { require.NoError(t, db.Close()) }()

	_, err = db.Exec("BEGIN EXCLUSIVE; COMMIT;")
	if isNotADatabase(err) {
		return
	}
	assert.NoError(t, err, "another connection to %s is open", filepath.Base(path))
}

func isNotADatabase(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && e.Code()&0xff == sqlite3.SQLITE_NOTADB
}

// assertRefused checks the guarantees of an Open that returned an error.
func assertRefused(t testing.TB, path, digestBefore string, db any, closeFn func() error) {
	t.Helper()
	assert.Nil(t, db)
	assert.Nil(t, closeFn)
	assert.Equal(t, digestBefore, digest(t, path), "file changed")
	assertNoSidecars(t, path)
	assertNoOpenConnections(t, path)
}

func schemaError(t testing.TB, err error) *database.SchemaError {
	t.Helper()
	var se *database.SchemaError
	require.True(t, errors.As(err, &se), "%v is not a *SchemaError", err)
	return se
}

// backupName returns the documented backup file name
// <path>.<YYYYMMDDTHHMMSS>.v<version>.bak.
func backupName(path string, version uint, at time.Time) string {
	return fmt.Sprintf("%s.%s.v%d.bak", path, at.Format("20060102T150405"), version)
}

// backups returns the backup files of the database at path, in name order.
func backups(t testing.TB, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	var paths []string
	prefix := filepath.Base(path) + "."
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".bak") {
			paths = append(paths, filepath.Join(filepath.Dir(path), e.Name()))
		}
	}
	return paths
}

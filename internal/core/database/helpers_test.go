package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sqlite3 "modernc.org/sqlite/lib"
)

// rawDB opens or creates path without going through the db service, with a
// single connection and no pragmas besides a busy timeout. The connection is
// closed at the end of the test; close it earlier before calling the db service.
func rawDB(t testing.TB, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driverName, fileURI(path)+"?mode=rwc&_pragma=busy_timeout(5000)")
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
	for _, sidecar := range sidecarPaths(path) {
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
	db, err := sql.Open(driverName, fileURI(path)+"?mode=rw&_pragma=locking_mode(EXCLUSIVE)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer func() { require.NoError(t, db.Close()) }()

	_, err = db.Exec("BEGIN EXCLUSIVE; COMMIT;")
	if sqliteCode(err) == sqlite3.SQLITE_NOTADB {
		return
	}
	assert.NoError(t, err, "another connection to %s is open", filepath.Base(path))
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

func schemaError(t testing.TB, err error) *SchemaError {
	t.Helper()
	var se *SchemaError
	require.True(t, errors.As(err, &se), "%v is not a *SchemaError", err)
	return se
}

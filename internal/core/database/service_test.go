package database_test

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/skarabasakis/subtill/internal/core/database/migrations"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

var testPreferences = domain.Preferences{
	Currency:               ptr("EUR"),
	CurrencyConversionMode: ptr(domain.CurrencyConversionModeLive),
}

func newDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subtill.db")
	require.NoError(t, database.Create(path, testPreferences))
	return path
}

const (
	migrationBase = `
CREATE TABLE preferences (
    id                       INTEGER PRIMARY KEY CHECK (id = 1),
    currency                 TEXT NOT NULL,
    currency_conversion_mode TEXT NOT NULL
);
CREATE TABLE parents (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE children (
    id        INTEGER PRIMARY KEY,
    parent_id INTEGER NOT NULL REFERENCES parents (id) ON DELETE CASCADE
);`
	migrationAddColumn  = `ALTER TABLE parents ADD COLUMN note TEXT;`
	migrationFailing    = `CREATE TABLE broken (id INTEGER PRIMARY KEY); INSERT INTO missing_table VALUES (1);`
	migrationFKViolates = `INSERT INTO children (id, parent_id) VALUES (1000, 999);`
	migrationRebuild    = `
CREATE TABLE new_parents (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
INSERT INTO new_parents (id, name) SELECT id, COALESCE(name, '') FROM parents;
DROP TABLE parents;
ALTER TABLE new_parents RENAME TO parents;`
)

// migrationsFS numbers ups as versions 1, 2, ...
func migrationsFS(ups ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for i, up := range ups {
		name := fmt.Sprintf("%06d_m%d", i+1, i+1)
		fsys[name+".up.sql"] = &fstest.MapFile{Data: []byte(up)}
		fsys[name+".down.sql"] = &fstest.MapFile{}
	}
	return fsys
}

func newService(t *testing.T, ups ...string) *database.Service {
	t.Helper()
	s, err := database.New(migrationsFS(ups...), database.WithBusyTimeout(100*time.Millisecond))
	require.NoError(t, err)
	return s
}

// seededV1 returns a database at version 1 with two parents and two children.
func seededV1(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subtill.db")
	require.NoError(t, newService(t, migrationBase).Create(path, testPreferences))
	database.RawExec(t, path,
		`INSERT INTO parents (id, name) VALUES (1, 'a'), (2, 'b')`,
		`INSERT INTO children (id, parent_id) VALUES (10, 1), (11, 2)`)
	return path
}

func TestSchemaVersion(t *testing.T) {
	names, err := fs.Glob(migrations.FS, "*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, names)

	var highest uint64
	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		require.True(t, ok, name)
		v, err := strconv.ParseUint(prefix, 10, 64)
		require.NoError(t, err, name)
		highest = max(highest, v)
	}
	assert.Equal(t, uint(highest), database.SchemaVersion)
}

func TestNew(t *testing.T) {
	t.Run("schema version is the newest migration", func(t *testing.T) {
		s, err := database.New(migrationsFS(migrationBase, migrationAddColumn, migrationRebuild))
		require.NoError(t, err)
		assert.Equal(t, uint(3), s.SchemaVersion())
	})

	t.Run("versions need not be contiguous", func(t *testing.T) {
		s, err := database.New(fstest.MapFS{
			"000001_a.up.sql": &fstest.MapFile{},
			"000007_b.up.sql": &fstest.MapFile{},
		})
		require.NoError(t, err)
		assert.Equal(t, uint(7), s.SchemaVersion())
	})

	t.Run("no migrations", func(t *testing.T) {
		s, err := database.New(fstest.MapFS{})
		assert.Error(t, err)
		assert.Nil(t, s)
	})
}

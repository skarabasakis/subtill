// Package migrations embeds the SQL migrations of the subtill schema.
package migrations

import "embed"

// FS holds the migration files at its root, in golang-migrate naming
// (<version>_<title>.up.sql and <version>_<title>.down.sql).
//
//go:embed *.sql
var FS embed.FS

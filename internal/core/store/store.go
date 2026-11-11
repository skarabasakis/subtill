// Package store holds the repositories that read and write subtill database
// tables. Every repository method takes a sqlx.Ext as its first argument, so it
// runs either directly on a pool or inside a transaction.
package store

import (
	"database/sql"
	"fmt"
)

// ErrNotFound is returned by Get* methods when no row matches.
var ErrNotFound = fmt.Errorf("not found: %w", sql.ErrNoRows)

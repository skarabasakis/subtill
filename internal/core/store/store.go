// Package store holds stateless types that read and write subtill database tables.
// Each type is a plural collection name matching its table (for example
// Subscriptions, Preferences) with no Repository suffix; singular names are
// reserved for domain derived models and *Record row types. Every method takes a
// sqlx.Ext as its first argument so it runs on a pool or inside a transaction.
// Callers qualify types as store.Subscriptions{}, store.Preferences{}, and avoid
// import aliases that hide the store package.
package store

import (
	"database/sql"
	"fmt"
)

// ErrNotFound is returned by Get* methods when no row matches.
var ErrNotFound = fmt.Errorf("not found: %w", sql.ErrNoRows)

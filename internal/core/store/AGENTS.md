# store package

Rules for changing `internal/core/store`. Run `go doc -all ./internal/core/store` for the API.

## Purpose

Stateless, table-scoped data access. This is the only package that writes SQL. Business rules, validation and derived fields (status, costs, payments) belong to the services, which compose these methods.

## Design principles

- **Records only.** Methods take and return `domain.*Record` types, never derived domain models. Types are plural collection nouns named after their table, with no `Repository` suffix.
- **Caller owns transactions.** Every method takes `sqlx.Ext` first, so it runs on the pool or inside a transaction. Operations that write more than once (subscription start, edit and cancel, preferences currency change) must run all their calls in one transaction.
- **SQL filters only what SQL can know.** `Subscriptions.Count` and `ListAll` apply `SubscriptionFiltersDb` in SQL. Filters on derived fields and all sorting stay in the service, so repositories take no sort arguments. The SQL status rules must match the Go derivation in [subscription derived fields](../../../docs/adr/20261018-subscription-derived-fields.md).
- **Statement shape is fixed.** Lists are bound as one JSON array and read with `json_each`, so a statement never changes with input size.
- **One query per association load.** Tags and billing periods are loaded together in a single query for one or many subscriptions: no N+1, and no join that multiplies rows. Each table owns its aggregate fragment and sort order, shared with its own list method; `Subscriptions` only composes them. Tags are only populated by the association loaders.
- **Columns are listed once.** Each repository declares `<table>Columns` as a list of bare column names, one per line; helpers quote and join them. Every full-record read, and the JSON aggregates, are built from it, so a new column is added in one place plus the row type. Reads never use `SELECT *`. Writes stay hand-written, and narrower projections stay inline.
- **Missing rows are cheap to detect.** `Get*` returns `ErrNotFound`. `Update*` and `Delete*` return rows affected, so services detect a missing row without an extra read.
- **Domain types stay free of DB concerns.** Dates are `civil.Date`, stored as `YYYY-MM-DD` text, and converted through unexported row types. Empty strings are stored as `NULL` and read back as empty.
- **Callers supply IDs.** Services generate them with `domain.NewID()` (UUID v7) before inserting. Being time-ordered is what makes `ListAll`'s `id DESC` mean newest first. The schema rejects empty IDs, and a caller-supplied ID makes retrying an insert safe.
- **Case-insensitivity comes from the schema.** Names and tags compare through their `COLLATE NOCASE` columns. Only substring matching, which collations don't cover, lower-cases explicitly.

## Testing

External `store_test` package against a real database from `database.Create` in `t.TempDir()`, without mocks. Fixtures are inserted through the store API. Filter tests check that `Count` agrees with `ListAll`.

# store package

Rules for changing `internal/core/store`. Run `go doc -all ./internal/core/store` for the API.

## Purpose

Stateless, table-scoped data access. This is the only package that writes SQL. Business rules, validation and derived fields (status, costs, payments) belong to the services, which compose these methods.

## Design principles

- **Records only.** Repositories take and return `domain.*Record` types, never derived domain models. They are named after their table as plural nouns, with no `Repository` suffix.
- **Caller owns transactions.** Every method accepts either the pool or a transaction, so services can group the writes of one operation into a single transaction.
- **SQL filters only what SQL can know.** Filters on stored columns run in SQL. Filters on derived fields, and all sorting, stay in the services, so repositories take no sort arguments. Any SQL that mirrors a derivation must match the Go rules in [subscription derived fields](../../../docs/adr/20261018-subscription-derived-fields.md).
- **Statement shape is fixed.** Variable-length inputs are bound as a single JSON array, so a statement's text never depends on input size.
- **One query per association load.** Associations are loaded for one or many parents in a single query: no N+1, and no join that multiplies rows. Each table owns its aggregate fragment and sort order; the parent repository only composes them.
- **Columns are listed once.** Each repository declares its column list once and builds full-record reads and aggregates from it, so adding a column touches only that list and the row type. Reads never use `SELECT *`.
- **Missing rows are cheap to detect.** Reads signal `ErrNotFound`; updates and deletes report rows affected, so services detect a missing row without an extra read.
- **Domain types stay free of DB concerns.** Storage representations (dates as text, empty strings as `NULL`) are converted through unexported row types and never leak into `domain`.
- **Repositories own their table's code; `helpers.go` holds only what is table-agnostic.** Anything that names a table, column or record type stays in its repository file. `helpers.go` is for code that is general enough to serve any repository, such as query execution, SQL building and storage-format conversions, even if only one repository uses it today.
- **Callers supply IDs.** Services generate time-ordered IDs before inserting, which makes ordering by ID mean newest first and makes retrying an insert safe.
- **Case-insensitivity comes from the schema.** Comparisons rely on `COLLATE NOCASE` columns; only substring matching, which collations don't cover, lower-cases explicitly.

## Testing

External `store_test` package against a real database in a temp directory, without mocks. Fixtures are inserted through the store API. SQL filters are checked for consistency between counting and listing.

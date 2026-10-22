# DB Service

The db service uses `database/sql` directly, not `sqlx`.

The database service exposes a constant `DB_VERSION` that represents the current schema version.

The database service exposes the following methods:

- func Create(path string, preferences *Preferences, err error)
Creates a blank sqlite database file under the given path, erroring if the file already exists.
Opens a connection to the database.
Migrates the database to the current schema version.
Then sets the preferences to the values from the parameters
Finally, it closes the connection to the database.

- func Open(path string) (db *sql.DB, err error)
Returns a connection to the database at the given path.

- func Close(db *sql.DB) (err error)
Closes the connection to the database.

- func GetInfo(db *sql.DB) (info *DBInfo, err error)
Returns information about the database, including the schema version, the total number of recorded subscriptions, and the preferences.

- func NeedsMigration(db *sql.DB) (needs bool, err error)
Returns true if the database schema version is not equal to the current schema version.

- func CanMigrate(db *sql.DB) (can bool, err error)
Returns true if the database schema version is less than the current schema version.

- func Migrate(db *sql.DB) (err error)
Migrates up the database to the current schema version.
Wrapper for the `golang-migrate/migrate` package.

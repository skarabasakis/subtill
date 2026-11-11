package database

var (
	RawDB                   = rawDB
	RawExec                 = rawExec
	RawQueryInt             = rawQueryInt
	RawSchemaState          = rawSchemaState
	Digest                  = digest
	AssertNoSidecars        = assertNoSidecars
	AssertNoOpenConnections = assertNoOpenConnections
	AssertRefused           = assertRefused
	SchemaErrorOf           = schemaError
	BackupPath              = backupPath
	NewestBackup            = newestBackup
)

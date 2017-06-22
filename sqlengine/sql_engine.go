package sqlengine

import "github.com/AusDTO/pe-rds-broker/config"

type SQLEngine interface {
	Open(conf config.DBConfig) error
	Close()
	ExistsDB(dbname string) (bool, error)
	CreateDB(dbname string) error
	DropDB(dbname string) error
	CreateUser(username string, password string) error
	DropUser(username string) error
	GrantPrivileges(dbname string, username string) error
	RevokePrivileges(dbname string, username string) error
	SetExtensions(extensions []string) error
	URI(dbname string, username string, password string) string
	JDBCURI(dbname string, username string, password string) string
	Config() config.DBConfig

	// CreateUsername returns an appropriate username for an app to use for a given instance.
	// Often this is random, but same databases need specific behaviour.
	CreateUsername(instanceID string) (string, error)
}

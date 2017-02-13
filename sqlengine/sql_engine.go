package sqlengine

import "github.com/AusDTO/pe-rds-broker/config"

type SQLEngine interface {
	Open(address string, port int64, dbname string, username string, password string, sslmode config.SSLMode) error
	Close()
	ExistsDB(dbname string) (bool, error)
	CreateDB(dbname string) error
	DropDB(dbname string) error
	CreateUser(username string, password string) error
	DropUser(username string) error
	GrantPrivileges(dbname string, username string) error
	RevokePrivileges(dbname string, username string) error
	URI(dbname string, username string, password string) string
	JDBCURI(dbname string, username string, password string) string
	Address() string
	Port() int64
}

func OpenConf(sqlEngine SQLEngine, config *config.DBConfig) error {
	return sqlEngine.Open(config.Url, config.Port, config.DBName, config.Username, config.Password, config.Sslmode)
}


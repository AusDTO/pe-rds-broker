package config

import (
	"os"
	"errors"
	"encoding/hex"
	"fmt"
	"strconv"
)

type SSLMode string
const (
	Disable = "disable"
	RequireNoVerify = "require"
	Verify = "verify-full"
)

//   Valid SSL modes:
//    * disable - No SSL
//    * require - Always SSL (skip verification)
//    * verify-full - Always SSL (require verification)
type DBConfig struct {
	DBType   string
	Url      string
	Username string
	Password string
	DBName   string
	Sslmode  SSLMode
	Port     int64
}


type EnvConfig struct {
	EncryptionKey []byte
	InternalDBConfig *DBConfig
	SharedPostgresDBConfig *DBConfig
	SharedMysqlDBConfig *DBConfig
}

func LoadEnvConfig() (*EnvConfig, error) {
	var config EnvConfig
	var err error
	config.SharedPostgresDBConfig, err = loadDBEnvConfig("SHARED_POSTGRES", 5432)
	if err != nil {
		return &config, err
	}
	config.SharedMysqlDBConfig, err = loadDBEnvConfig("SHARED_MYSQL", 3306)
	if err != nil {
		return &config, err
	}
	config.InternalDBConfig, err = loadDBEnvConfig("INTERNAL", 5432)
	if err != nil {
		return &config, err
	}
	config.InternalDBConfig.DBType = os.Getenv("RDSBROKER_INTERNAL_DB_PROVIDER")
	if config.InternalDBConfig.DBType != "postgres" && config.InternalDBConfig.DBType != "sqlite3" {
		return &config, errors.New("Unknown internal DB provider")
	}
	config.EncryptionKey, err = hex.DecodeString(os.Getenv("RDSBROKER_ENCRYPTION_KEY"))
	if err != nil {
		return &config, fmt.Errorf("Failed to parse RDSBROKER_ENCRYPTION_KEY", err)
	}
	if len(config.EncryptionKey) != 32 {
		return &config, errors.New("RDSBROKER_ENCRYPTION_KEY must be a hex-encoded 256-bit key")
	}
	return &config, nil
}

func loadDBEnvConfig(version string, defaultPort int64) (*DBConfig, error) {
	var dbconfig DBConfig
	var err error
	dbconfig.DBName   = os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_NAME", version))
	dbconfig.Username = os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_USERNAME", version))
	dbconfig.Password = os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_PASSWORD", version))
	dbconfig.Url      = os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_URL", version))
	dbconfig.Sslmode  = SSLMode(os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_SSLMODE", version)))
	port_str := os.Getenv(fmt.Sprintf("RDSBROKER_%s_DB_PORT"))
	if port_str != "" {
		dbconfig.Port, err = strconv.ParseInt(port_str, 0, 64)
		if err != nil {
			return &dbconfig, errors.New(fmt.Sprintf("Invalid port in environment variable RDSBROKER_%s_DB_PORT", version))
		}
	} else {
		dbconfig.Port = defaultPort
	}
	return &dbconfig, nil
}

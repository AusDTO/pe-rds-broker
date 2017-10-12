package config

import (
	"errors"
	"fmt"
	"strconv"

	cfcommon "github.com/govau/cf-common"
)

type SSLMode string

const (
	Disable         = "disable"
	RequireNoVerify = "require"
	Verify          = "verify-full"
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
	Username               string
	Password               string
	EncryptionKey          []byte
	InternalDBConfig       *DBConfig
	SharedPostgresDBConfig *DBConfig
	SharedMysqlDBConfig    *DBConfig
}

func MustLoadEnvConfig(envVars *cfcommon.EnvVars) *EnvConfig {
	config := &EnvConfig{
		Username:      envVars.MustString("RDSBROKER_USERNAME"),
		Password:      envVars.MustString("RDSBROKER_PASSWORD"),
		EncryptionKey: envVars.MustHexEncodedByteArray("RDSBROKER_ENCRYPTION_KEY", 32),

		SharedPostgresDBConfig: mustLoadDBConfig(envVars, "SHARED_POSTGRES", 5432),
		SharedMysqlDBConfig:    mustLoadDBConfig(envVars, "SHARED_MYSQL", 3306),

		InternalDBConfig: mustLoadDBConfig(envVars, "INTERNAL", 5432),
	}

	config.InternalDBConfig.DBType = envVars.MustString("RDSBROKER_INTERNAL_DB_PROVIDER")
	if config.InternalDBConfig.DBType != "postgres" && config.InternalDBConfig.DBType != "sqlite3" {
		panic(errors.New("Unknown internal DB provider"))
	}

	return config
}

func mustLoadDBConfig(envVar *cfcommon.EnvVars, version string, defaultPort int) *DBConfig {
	port, err := strconv.Atoi(envVar.String(fmt.Sprintf("RDSBROKER_%s_DB_PORT", version), strconv.Itoa(defaultPort)))
	if err != nil {
		panic(err)
	}

	return &DBConfig{
		DBName:   envVar.MustString(fmt.Sprintf("RDSBROKER_%s_DB_NAME", version)),
		Url:      envVar.MustString(fmt.Sprintf("RDSBROKER_%s_DB_URL", version)),
		Port:     int64(port),
		Sslmode:  SSLMode(envVar.String(fmt.Sprintf("RDSBROKER_%s_DB_SSLMODE", version), "")),
		Username: envVar.MustString(fmt.Sprintf("RDSBROKER_%s_DB_USERNAME", version)),
		Password: envVar.MustString(fmt.Sprintf("RDSBROKER_%s_DB_PASSWORD", version)),
	}
}

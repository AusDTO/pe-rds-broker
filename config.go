package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/AusDTO/pe-rds-broker/rdsbroker"
	"encoding/hex"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"strconv"
)

type Config struct {
	LogLevel  string           `yaml:"log_level"`
	Username  string           `yaml:"username"`
	Password  string           `yaml:"password"`
	RDSConfig rdsbroker.Config `yaml:"rds_config"`
}

func LoadConfig(configFile string) (config *Config, err error) {
	if configFile == "" {
		return config, errors.New("Must provide a config file")
	}

	file, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return config, err
	}

	if err = yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

	if err = config.Validate(); err != nil {
		return config, fmt.Errorf("Validating config contents: %s", err)
	}

	return config, nil
}

func (c Config) Validate() error {
	if c.LogLevel == "" {
		return errors.New("Must provide a non-empty LogLevel")
	}

	if c.Username == "" {
		return errors.New("Must provide a non-empty Username")
	}

	if c.Password == "" {
		return errors.New("Must provide a non-empty Password")
	}

	if err := c.RDSConfig.Validate(); err != nil {
		return fmt.Errorf("Validating RDS configuration: %s", err)
	}

	return nil
}

type EnvConfig struct {
	EncryptionKey []byte
	InternalDBConfig internaldb.DBConfig
}

func LoadEnvConfig() (*EnvConfig, error) {
	var config EnvConfig
	var err error
	config.InternalDBConfig.DBName = os.Getenv("RDSBROKER_INTERNAL_DB_NAME")
	if config.InternalDBConfig.DBName == "" {
		return &config, errors.New("RDSBROKER_INTERNAL_DB_NAME cannot be empty")
	}
	config.InternalDBConfig.DBType = os.Getenv("RDSBROKER_INTERNAL_DB_PROVIDER")
	if config.InternalDBConfig.DBType != "postgres" && config.InternalDBConfig.DBType != "sqlite3" {
		return &config, errors.New("Unknown internal DB provider")
	}
	config.InternalDBConfig.Username = os.Getenv("RDSBROKER_INTERNAL_DB_USERNAME")
	config.InternalDBConfig.Password = os.Getenv("RDSBROKER_INTERNAL_DB_PASSWORD")
	config.InternalDBConfig.Url = os.Getenv("RDSBROKER_INTERNAL_DB_URL")
	config.InternalDBConfig.Sslmode = os.Getenv("RDSBROKER_INTERNAL_DB_SSLMODE")
	port_str := os.Getenv("RDSBROKER_INTERNAL_DB_PORT")
	if port_str != "" {
		config.InternalDBConfig.Port, err = strconv.Atoi(port_str)
		if err != nil {
			return &config, errors.New("Invalid port in environment variable RDSBROKER_INTERNAL_DB_PORT")
		}
	} else {
		config.InternalDBConfig.Port = 5432
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

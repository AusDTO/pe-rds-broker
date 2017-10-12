package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/AusDTO/pe-rds-broker/rdsbroker"
	cfcommon "github.com/govau/cf-common"
)

type Config struct {
	LogLevel  string           `yaml:"log_level"`
	RDSConfig rdsbroker.Config `yaml:"rds_config"`
}

func LoadConfig(envVars *cfcommon.EnvVars, configFile string) (config *Config, err error) {
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

	config.RDSConfig.DBPrefix = envVars.MustString("DB_PREFIX")
	for _, service := range config.RDSConfig.Catalog.Services {
		for _, sp := range service.Plans {
			sp.RDSProperties.MultiAZ = envVars.Bool("DB_MULTI_AZ")
			sp.RDSProperties.DBSubnetGroupName = envVars.MustString("DB_RDS_SUBNET_GROUP_NAME")
			sp.RDSProperties.DBSecurityGroups = []string{envVars.MustString(fmt.Sprintf("DB_RDS_SECURITY_GROUP_%s", strings.ToUpper(sp.RDSProperties.Engine))}
		}
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

	if err := c.RDSConfig.Validate(); err != nil {
		return fmt.Errorf("Validating RDS configuration: %s", err)
	}

	return nil
}

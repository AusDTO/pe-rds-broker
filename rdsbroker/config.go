package rdsbroker

import (
	"errors"
	"fmt"
	"regexp"
)

type Config struct {
	Region                       string  `yaml:"region"`
	DBPrefix                     string  `yaml:"db_prefix"`
	AllowUserProvisionParameters bool    `yaml:"allow_user_provision_parameters"`
	AllowUserUpdateParameters    bool    `yaml:"allow_user_update_parameters"`
	AllowUserBindParameters      bool    `yaml:"allow_user_bind_parameters"`
	Catalog                      Catalog `yaml:"catalog"`
}

func (c Config) Validate() error {
	if c.Region == "" {
		return errors.New("Must provide a non-empty Region")
	}

	if c.DBPrefix == "" {
		return errors.New("Must provide a non-empty DBPrefix")
	}

	if !regexp.MustCompile("^[[:alpha:]][[:alnum:]]*$").MatchString(c.DBPrefix) {
		return errors.New("DBPrefix must begin with a letter and contain only alphanumeric characters")
	}

	if err := c.Catalog.Validate(); err != nil {
		return fmt.Errorf("Validating Catalog configuration: %s", err)
	}

	return nil
}

package main

import (
	"flag"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"

	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"github.com/AusDTO/pe-rds-broker/utils"
)

var (
	logLevel   string
	instanceID string
)

func init() {
	flag.StringVar(&logLevel, "log", "INFO", "Log level (DEBUG, INFO, ERROR or FATAL)")
	flag.StringVar(&instanceID, "instance", "", "Instance to decrypt passwords for")
}

func main() {
	flag.Parse()
	if instanceID == "" {
		fmt.Println("missing instance")
		flag.Usage()
		os.Exit(1)
	}
	logger := utils.BuildLogger(logLevel, "rds-broker.decrypt")

	envConfig, err := config.LoadEnvConfig()
	if err != nil {
		logger.Fatal("load-environment", err)
	}

	internalDB, err := internaldb.DBInit(envConfig.InternalDBConfig, logger)
	if err != nil {
		logger.Fatal("connectdb", err)
	}

	instance := internaldb.FindInstance(internalDB, instanceID)
	if instance == nil {
		logger.Fatal("load-instance", fmt.Errorf("Instance '%s' not found", instanceID))
	}
	for _, user := range instance.Users {
		password, err := user.Password(envConfig.EncryptionKey)
		if err != nil {
			logger.Fatal("decrypt", err, lager.Data{"user_id": user.ID})
		}
		fmt.Printf("%s user %s: %s\n", user.Type, user.Username, password)
	}
}

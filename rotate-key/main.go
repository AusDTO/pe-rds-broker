package main

import (
	"flag"
	"os"

	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/AusDTO/pe-rds-broker/utils"
)

var (
	logLevel string
	failFast bool
)

func init() {
	flag.StringVar(&logLevel, "log", "INFO", "Log level (DEBUG, INFO, ERROR or FATAL)")
	flag.BoolVar(&failFast, "fast", false, "Whether to fail on first error or attempt to continue")
}

func main() {
	flag.Parse()

	logger := utils.BuildLogger(logLevel, "rds-broker.rotatekey")

	envConfig, err := config.LoadEnvConfig()
	if err != nil {
		logger.Fatal("load-environment", err)
	}

	internalDB, err := internaldb.DBInit(envConfig.InternalDBConfig, logger)
	if err != nil {
		logger.Fatal("connectdb", err)
	}

	old_encryption_key, err := hex.DecodeString(os.Getenv("RDSBROKER_ENCRYPTION_KEY_OLD"))
	if err != nil {
		logger.Fatal("parse-RDSBROKER_ENCRYPTION_KEY_OLD", err)
	}
	if len(old_encryption_key) != 32 {
		logger.Fatal("parse-RDSBROKER_ENCRYPTION_KEY_OLD", errors.New("RDSBROKER_ENCRYPTION_KEY_OLD must be a hex-encoded 256-bit key"))
	}

	err = internaldb.RotateKey(internalDB, old_encryption_key, envConfig.EncryptionKey, logger, failFast)
	if err != nil {
		logger.Fatal("rotate-key", err)
	}
	fmt.Println("Successfully rotated the database encryption key")
}

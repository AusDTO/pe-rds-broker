package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/pivotal-cf/brokerapi"

	"github.com/AusDTO/pe-rds-broker/awsrds"
	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"github.com/AusDTO/pe-rds-broker/rdsbroker"
	"github.com/AusDTO/pe-rds-broker/sqlengine"
)

var (
	configFilePath string
	port           string

	logLevels = map[string]lager.LogLevel{
		"DEBUG": lager.DEBUG,
		"INFO":  lager.INFO,
		"ERROR": lager.ERROR,
		"FATAL": lager.FATAL,
	}
)

func init() {
	flag.StringVar(&configFilePath, "config", "", "Location of the config file")
	flag.StringVar(&port, "port", "3000", "Listen port")
}

func buildLogger(logLevel string) lager.Logger {
	laggerLogLevel, ok := logLevels[strings.ToUpper(logLevel)]
	if !ok {
		log.Fatal("Invalid log level: ", logLevel)
	}

	logger := lager.NewLogger("rds-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, laggerLogLevel))

	return logger
}

func main() {
	flag.Parse()

	configYml, err := LoadConfig(configFilePath)
	if err != nil {
		log.Fatalf("Error loading config file: %s", err)
	}

	logger := buildLogger(configYml.LogLevel)

	envConfig, err := config.LoadEnvConfig()
	if err != nil {
		logger.Fatal("load-environment", err)
	}

	awsConfig := aws.NewConfig().WithRegion(configYml.RDSConfig.Region)
	awsSession := session.New(awsConfig)

	iamsvc := iam.New(awsSession)
	rdssvc := rds.New(awsSession)
	dbInstance := awsrds.NewRDSDBInstance(configYml.RDSConfig.Region, iamsvc, rdssvc, logger)
	dbCluster := awsrds.NewRDSDBCluster(configYml.RDSConfig.Region, iamsvc, rdssvc, logger)

	sqlProvider := sqlengine.NewProviderService(logger)

	internalDB, err := internaldb.DBInit(envConfig.InternalDBConfig, logger)
	if err != nil {
		logger.Fatal("connectdb", err)
	}

	sharedPostgres, err := sqlProvider.GetSQLEngine("postgres")
	if err != nil {
		logger.Fatal("get-postgres-engine", err)
	}
	err = sharedPostgres.Open(*envConfig.SharedPostgresDBConfig)
	if err != nil {
		logger.Fatal("connect-shared-postgres", err)
	}

	sharedMysql, err := sqlProvider.GetSQLEngine("mysql")
	if err != nil {
		logger.Fatal("get-mysql-engine", err)
	}
	err = sharedMysql.Open(*envConfig.SharedMysqlDBConfig)
	if err != nil {
		logger.Fatal("connect-shared-mysql", err)
	}

	serviceBroker := rdsbroker.New(configYml.RDSConfig, dbInstance, dbCluster, sqlProvider, logger, internalDB, sharedPostgres, sharedMysql, envConfig.EncryptionKey)

	credentials := brokerapi.BrokerCredentials{
		Username: envConfig.Username,
		Password: envConfig.Password,
	}

	brokerAPI := brokerapi.New(serviceBroker, logger, credentials)
	http.Handle("/", brokerAPI)

	logger.Info("RDS Service Broker started on port " + port + "...")
	logger.Fatal("listen-serve", http.ListenAndServe(":"+port, nil))
}

package main

import (
	"flag"
	"log"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/pivotal-cf/brokerapi"

	"github.com/AusDTO/pe-rds-broker/awsrds"
	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"github.com/AusDTO/pe-rds-broker/rdsbroker"
	"github.com/AusDTO/pe-rds-broker/sqlengine"
	"github.com/AusDTO/pe-rds-broker/utils"
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

func main() {
	flag.Parse()

	configYml, err := LoadConfig(configFilePath)
	if err != nil {
		log.Fatalf("Error loading config file: %s", err)
	}

	logger := utils.BuildLogger(configYml.LogLevel, "rds-broker")

	envConfig, err := config.LoadEnvConfig()
	if err != nil {
		logger.Fatal("load-environment", err)
	}

	awsConfig := aws.NewConfig().WithRegion(configYml.RDSConfig.Region)
	awsSession := session.New(awsConfig)

	rdssvc := rds.New(awsSession)
	dbInstance := awsrds.NewRDSDBInstance(configYml.RDSConfig.Region, rdssvc, logger)
	dbCluster := awsrds.NewRDSDBCluster(configYml.RDSConfig.Region, rdssvc, logger)

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

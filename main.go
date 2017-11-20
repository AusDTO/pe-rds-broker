package main

import (
	"log"
	"net/http"

	cfcommon "github.com/govau/cf-common"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
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

func main() {
	envVar := cfcommon.NewDefaultEnvLookup()

	port := envVar.MustString("PORT")

	configYml, err := LoadConfig(envVar, envVar.String("CONFIG_PATH", "config.yml"))
	if err != nil {
		log.Fatalf("Error loading config file: %s", err)
	}

	logger := utils.BuildLogger(configYml.LogLevel, "rds-broker")

	envConfig := config.MustLoadEnvConfig(envVar)

	awsConfig := aws.NewConfig().WithRegion(configYml.RDSConfig.Region).WithCredentials(
		credentials.NewStaticCredentials(
			envVar.MustString("AWS_ACCESS_KEY_ID"),
			envVar.MustString("AWS_SECRET_ACCESS_KEY"),
		),
	)
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

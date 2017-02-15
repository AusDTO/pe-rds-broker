package rdsbroker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
	"github.com/mitchellh/mapstructure"
	"github.com/pivotal-cf/brokerapi"

	"github.com/AusDTO/pe-rds-broker/awsrds"
	"github.com/AusDTO/pe-rds-broker/sqlengine"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"github.com/AusDTO/pe-rds-broker/utils"
	"github.com/AusDTO/pe-rds-broker/config"
)

const instanceIDLogKey = "instance-id"
const bindingIDLogKey = "binding-id"
const detailsLogKey = "details"
const asyncAllowedLogKey = "async-allowed"

var rdsStatus2State = map[string]brokerapi.LastOperationState{
	"available":                    brokerapi.Succeeded,
	"backing-up":                   brokerapi.InProgress,
	"creating":                     brokerapi.InProgress,
	"deleting":                     brokerapi.InProgress,
	"maintenance":                  brokerapi.InProgress,
	"modifying":                    brokerapi.InProgress,
	"rebooting":                    brokerapi.InProgress,
	"renaming":                     brokerapi.InProgress,
	"resetting-master-credentials": brokerapi.InProgress,
	"upgrading":                    brokerapi.InProgress,
}

type RDSBroker struct {
	dbPrefix                     string
	allowUserProvisionParameters bool
	allowUserUpdateParameters    bool
	allowUserBindParameters      bool
	catalog                      Catalog
	dbInstance                   awsrds.DBInstance
	dbCluster                    awsrds.DBCluster
	sqlProvider                  sqlengine.Provider
	logger                       lager.Logger
	internalDB                   *gorm.DB
	sharedEngines                map[string]sqlengine.SQLEngine
	encryptionKey                []byte
}

func New(
	config Config,
	dbInstance awsrds.DBInstance,
	dbCluster awsrds.DBCluster,
	sqlProvider sqlengine.Provider,
	logger lager.Logger,
	internalDB *gorm.DB,
	sharedPostgres sqlengine.SQLEngine,
	sharedMysql sqlengine.SQLEngine,
	encryptionKey []byte,
) *RDSBroker {
	return &RDSBroker{
		dbPrefix:                     config.DBPrefix,
		allowUserProvisionParameters: config.AllowUserProvisionParameters,
		allowUserUpdateParameters:    config.AllowUserUpdateParameters,
		allowUserBindParameters:      config.AllowUserBindParameters,
		catalog:                      config.Catalog,
		dbInstance:                   dbInstance,
		dbCluster:                    dbCluster,
		sqlProvider:                  sqlProvider,
		logger:                       logger.Session("broker"),
		internalDB:                   internalDB,
		sharedEngines:                map[string]sqlengine.SQLEngine{"postgres": sharedPostgres, "mysql": sharedMysql},
		encryptionKey:                encryptionKey,
	}
}

func (b *RDSBroker) Services(context context.Context) []brokerapi.Service {
	b.logger.Debug("services")

	var services []brokerapi.Service

	/* Service and brokerapi.Service are slightly different data structures
	 * The easiest way to convert is via JSON
	 *
	 * TODO: Find a better way
	 */
	servicesStr, err := json.Marshal(b.catalog.Services)
	if err != nil {
		b.logger.Error("marshal-error", err)
		return services
	}

	if err = json.Unmarshal(servicesStr, &services); err != nil {
		b.logger.Error("unmarshal-error", err)
		return services
	}
	return services
}

func (b *RDSBroker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	b.logger.Debug("provision", lager.Data{
		instanceIDLogKey:   instanceID,
		detailsLogKey:      details,
		asyncAllowedLogKey: asyncAllowed,
	})

	provisionSpec := brokerapi.ProvisionedServiceSpec{IsAsync: true}

	if !asyncAllowed {
		return provisionSpec, brokerapi.ErrAsyncRequired
	}

	provisionParameters := ProvisionParameters{}
	if b.allowUserProvisionParameters && len(details.RawParameters) > 0 {
		if err := json.Unmarshal(details.RawParameters, &provisionParameters); err != nil {
			return provisionSpec, err
		}
	}

	servicePlan, ok := b.catalog.FindServicePlan(details.ServiceID, details.PlanID)
	if !ok {
		return provisionSpec, fmt.Errorf("Service Plan '%s' not found", details.PlanID)
	}

	// There's a potential race condition here but it's better than nothing
	if internaldb.FindInstance(b.internalDB, instanceID) != nil {
		return provisionSpec, errors.New("Instance already exists")
	}

	instance, err := internaldb.NewInstance(details.ServiceID, details.PlanID, instanceID, b.dbPrefix, b.encryptionKey)
	if err != nil {
		return provisionSpec, err
	}

	if servicePlan.RDSProperties.Shared {
		sqlEngine := b.sharedEngines[servicePlan.RDSProperties.Engine]
		err := sqlEngine.CreateDB(instance.DBName)
		if err != nil {
			return provisionSpec, err
		}
		provisionSpec.IsAsync = false
	} else {
		if strings.ToLower(servicePlan.RDSProperties.Engine) == "aurora" {
			createDBCluster := b.createDBCluster(instance, servicePlan, provisionParameters, details)
			if err = b.dbCluster.Create(b.dbClusterIdentifier(instance), *createDBCluster); err != nil {
				return provisionSpec, err
			}
			defer func() {
				if err != nil {
					b.dbCluster.Delete(b.dbClusterIdentifier(instance), servicePlan.RDSProperties.SkipFinalSnapshot)
				}
			}()
		}

		createDBInstance := b.createDBInstance(instance, servicePlan, provisionParameters, details)
		if err = b.dbInstance.Create(b.dbInstanceIdentifier(instance), *createDBInstance); err != nil {
			return provisionSpec, err
		}
	}

	if err = b.internalDB.Save(instance).Error; err != nil {
		// TODO rollback
		return provisionSpec, errors.New("RDS instance created but failed to save reference to local database")
	}

	return provisionSpec, nil
}

func (b *RDSBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	b.logger.Debug("update", lager.Data{
		instanceIDLogKey:   instanceID,
		detailsLogKey:      details,
		asyncAllowedLogKey: asyncAllowed,
	})

	updateSpec := brokerapi.UpdateServiceSpec{IsAsync: true}

	if !asyncAllowed {
		return updateSpec, brokerapi.ErrAsyncRequired
	}

	updateParameters := UpdateParameters{}
	if b.allowUserUpdateParameters {
		if err := mapstructure.Decode(details.Parameters, &updateParameters); err != nil {
			return updateSpec, err
		}
	}

	service, ok := b.catalog.FindService(details.ServiceID)
	if !ok {
		return updateSpec, fmt.Errorf("Service '%s' not found", details.ServiceID)
	}

	if !service.PlanUpdateable {
		return updateSpec, brokerapi.ErrPlanChangeNotSupported
	}

	servicePlan, ok := b.catalog.FindServicePlan(details.ServiceID, details.PlanID)
	if !ok {
		return updateSpec, fmt.Errorf("Service Plan '%s' not found", details.PlanID)
	}

	instance := internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		return updateSpec, errors.New("Unknown instance ID")
	}

	if strings.ToLower(servicePlan.RDSProperties.Engine) == "aurora" {
		modifyDBCluster := b.modifyDBCluster(instance, servicePlan, updateParameters, details)
		if err := b.dbCluster.Modify(b.dbClusterIdentifier(instance), *modifyDBCluster, updateParameters.ApplyImmediately); err != nil {
			return updateSpec, err
		}
	}

	modifyDBInstance := b.modifyDBInstance(instance, servicePlan, updateParameters, details)
	if err := b.dbInstance.Modify(b.dbInstanceIdentifier(instance), *modifyDBInstance, updateParameters.ApplyImmediately); err != nil {
		if err == awsrds.ErrDBInstanceDoesNotExist {
			return updateSpec, brokerapi.ErrInstanceDoesNotExist
		}
		return updateSpec, err
	}

	return updateSpec, nil
}

func (b *RDSBroker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.logger.Debug("deprovision", lager.Data{
		instanceIDLogKey:   instanceID,
		detailsLogKey:      details,
		asyncAllowedLogKey: asyncAllowed,
	})

	deprovisionSpec := brokerapi.DeprovisionServiceSpec{IsAsync: true}

	if !asyncAllowed {
		return deprovisionSpec, brokerapi.ErrAsyncRequired
	}

	servicePlan, ok := b.catalog.FindServicePlan(details.ServiceID, details.PlanID)
	if !ok {
		return deprovisionSpec, fmt.Errorf("Service Plan '%s' not found", details.PlanID)
	}

	instance := internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		return deprovisionSpec, errors.New("Unknown instance ID")
	}

	skipDBInstanceFinalSnapshot := servicePlan.RDSProperties.SkipFinalSnapshot
	if strings.ToLower(servicePlan.RDSProperties.Engine) == "aurora" {
		skipDBInstanceFinalSnapshot = true
	}

	if servicePlan.RDSProperties.Shared {
		sqlEngine := b.sharedEngines[servicePlan.RDSProperties.Engine]
		err := sqlEngine.DropDB(instance.DBName)
		if err != nil {
			return deprovisionSpec, err
		}
		err = instance.Delete(b.internalDB)
		if err != nil {
			// log an move on because the real database is gone
			b.logger.Error("delete-instance", err)
		}
		deprovisionSpec.IsAsync = false
	} else {
		if err := b.dbInstance.Delete(b.dbInstanceIdentifier(instance), skipDBInstanceFinalSnapshot); err != nil {
			if err == awsrds.ErrDBInstanceDoesNotExist {
				return deprovisionSpec, brokerapi.ErrInstanceDoesNotExist
			}
			return deprovisionSpec, err
		}

		if strings.ToLower(servicePlan.RDSProperties.Engine) == "aurora" {
			b.dbCluster.Delete(b.dbClusterIdentifier(instance), servicePlan.RDSProperties.SkipFinalSnapshot)
		}

		// We do not delete the internal reference to the DB here because we've only started the delete process
		// and we still need the reference for LastOperation()
	}

	return deprovisionSpec, nil
}

func (b *RDSBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	b.logger.Debug("bind", lager.Data{
		instanceIDLogKey: instanceID,
		bindingIDLogKey:  bindingID,
		detailsLogKey:    details,
	})

	binding := brokerapi.Binding{}

	bindParameters := BindParameters{}
	if b.allowUserBindParameters {
		if err := mapstructure.Decode(details.Parameters, &bindParameters); err != nil {
			return binding, err
		}
		if !utils.IsSimpleIdentifier(bindParameters.Username) {
			return binding, errors.New("Username must begin with a letter and contain only alphanumeric characters")
		}
	}

	service, ok := b.catalog.FindService(details.ServiceID)
	if !ok {
		return binding, fmt.Errorf("Service '%s' not found", details.ServiceID)
	}

	if !service.Bindable {
		return binding, errors.New("Service is not bindable")
	}

	servicePlan, ok := b.catalog.FindServicePlan(details.ServiceID, details.PlanID)
	if !ok {
		return binding, fmt.Errorf("Service Plan '%s' not found", details.PlanID)
	}

	instance := internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		return binding, errors.New("Unknown instance ID")
	}

	var sqlEngine sqlengine.SQLEngine
	if servicePlan.RDSProperties.Shared {
		sqlEngine = b.sharedEngines[servicePlan.RDSProperties.Engine]
	} else {
		var err error
		sqlEngine, err = b.dedicatedSqlEngine(instance, servicePlan.RDSProperties.Engine)
		if err != nil {
			return binding, err
		}
		defer sqlEngine.Close()
	}

	user, new, err := instance.Bind(b.internalDB, bindingID, b.dbUsername(bindParameters.Username, details.AppGUID), internaldb.Standard, b.encryptionKey)
	if err != nil {
		return binding, err
	}

	userPassword, err := user.Password(b.encryptionKey)
	if err != nil {
		return binding, err
	}

	if new {
		if err = sqlEngine.CreateUser(user.Username, userPassword); err != nil {
			return binding, err
		}

		if err = sqlEngine.GrantPrivileges(instance.DBName, user.Username); err != nil {
			return binding, err
		}
	}

	binding.Credentials = &CredentialsHash{
		Host:     sqlEngine.Address(),
		Port:     sqlEngine.Port(),
		Name:     instance.DBName,
		Username: user.Username,
		Password: userPassword,
		URI:      sqlEngine.URI(instance.DBName, user.Username, userPassword),
		JDBCURI:  sqlEngine.JDBCURI(instance.DBName, user.Username, userPassword),
	}

	return binding, nil
}

func (b *RDSBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	b.logger.Debug("unbind", lager.Data{
		instanceIDLogKey: instanceID,
		bindingIDLogKey:  bindingID,
		detailsLogKey:    details,
	})

	servicePlan, ok := b.catalog.FindServicePlan(details.ServiceID, details.PlanID)
	if !ok {
		return fmt.Errorf("Service Plan '%s' not found", details.PlanID)
	}

	instance := internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		return errors.New("Unknown instance ID")
	}

	user, delete, err := instance.Unbind(b.internalDB, bindingID)
	if err != nil {
		return err
	}

	if delete {
		var sqlEngine sqlengine.SQLEngine
		if servicePlan.RDSProperties.Shared {
			sqlEngine = b.sharedEngines[servicePlan.RDSProperties.Engine]
		} else {
			sqlEngine, err = b.dedicatedSqlEngine(instance, servicePlan.RDSProperties.Engine)
			if err != nil {
				return err
			}
			defer sqlEngine.Close()
		}

		if err = sqlEngine.RevokePrivileges(instance.DBName, user.Username); err != nil {
			return err
		}

		if err = sqlEngine.DropUser(user.Username); err != nil {
			return err
		}

		if err = user.Delete(b.internalDB); err != nil {
			// Log and move on because the actual user is gone
			b.logger.Error("delete-user", err)
		}
	}

	return nil
}

func (b *RDSBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	b.logger.Debug("last-operation", lager.Data{
		instanceIDLogKey: instanceID,
	})

	lastOperation := brokerapi.LastOperation{State: brokerapi.Failed}

	instance := internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		return lastOperation, errors.New("Unknown instance ID")
	}

	servicePlan, found := b.catalog.FindServicePlan(instance.ServiceID, instance.PlanID)
	if !found {
		return lastOperation, errors.New("Unknown service plan")
	}

	if servicePlan.RDSProperties.Shared {
		// shared instances don't have async operations
		return brokerapi.LastOperation{State: brokerapi.Failed, Description: "No last operation"}, nil
	}

	dbInstanceDetails, err := b.dbInstance.Describe(b.dbInstanceIdentifier(instance))
	if err != nil {
		if err == awsrds.ErrDBInstanceDoesNotExist {
			// The instance doesn't exist on AWS but we have a local reference to it
			// We should get rid of our local reference
			if err := instance.Delete(b.internalDB); err != nil {
				b.logger.Error("delete-internal", err)
			}

			return lastOperation, brokerapi.ErrInstanceDoesNotExist
		}
		return lastOperation, err
	}

	lastOperation.Description = fmt.Sprintf("DB Instance '%s' status is '%s'", b.dbInstanceIdentifier(instance), dbInstanceDetails.Status)

	if state, ok := rdsStatus2State[dbInstanceDetails.Status]; ok {
		lastOperation.State = state
	}

	if lastOperation.State == brokerapi.Succeeded && dbInstanceDetails.PendingModifications {
		lastOperation.State = brokerapi.InProgress
		lastOperation.Description = fmt.Sprintf("DB Instance '%s' has pending modifications", b.dbInstanceIdentifier(instance))
	}

	return lastOperation, nil
}

func (b *RDSBroker) dbClusterIdentifier(instance *internaldb.DBInstance) string {
	return fmt.Sprintf("%s-%s", b.dbPrefix, strings.Replace(instance.InstanceID, "_", "-", -1))
}

func (b *RDSBroker) dbInstanceIdentifier(instance *internaldb.DBInstance) string {
	return fmt.Sprintf("%s-%s", b.dbPrefix, strings.Replace(instance.InstanceID, "_", "-", -1))
}

// requestedUsername should have already been tested for validity
func (b *RDSBroker) dbUsername(requestedUsername, appID string) string {
	if requestedUsername != "" {
		return requestedUsername
	} else if appID != "" {
		return "u" + strings.Replace(appID, "-", "_", -1)
	} else {
		username, _ := utils.RandUsername()
		return username
	}
}

func (b *RDSBroker) dbConnInfo(instance *internaldb.DBInstance, engine string) (dbAddress, dbName string, dbPort int64, err error) {
	if strings.ToLower(engine) == "aurora" {
		var dbClusterDetails awsrds.DBClusterDetails
		dbClusterDetails, err = b.dbCluster.Describe(b.dbClusterIdentifier(instance))
		if err != nil {
			if err == awsrds.ErrDBInstanceDoesNotExist {
				err = brokerapi.ErrInstanceDoesNotExist
			}
			return
		}

		dbAddress = dbClusterDetails.Endpoint
		dbPort = dbClusterDetails.Port
		if dbClusterDetails.DatabaseName != "" {
			dbName = dbClusterDetails.DatabaseName
		} else {
			dbName = instance.DBName
		}
	} else {
		var dbInstanceDetails awsrds.DBInstanceDetails
		dbInstanceDetails, err = b.dbInstance.Describe(b.dbInstanceIdentifier(instance))
		if err != nil {
			if err == awsrds.ErrDBInstanceDoesNotExist {
				err = brokerapi.ErrInstanceDoesNotExist
			}
			return
		}

		dbAddress = dbInstanceDetails.Address
		dbPort = dbInstanceDetails.Port
		if dbInstanceDetails.DBName != "" {
			dbName = dbInstanceDetails.DBName
		} else {
			dbName = instance.DBName
		}
	}
	return
}

func (b *RDSBroker) dedicatedSqlEngine(instance *internaldb.DBInstance, engine string) (sqlEngine sqlengine.SQLEngine, err error) {
	dbAddress, dbName, dbPort, err := b.dbConnInfo(instance, engine)
	if err != nil {
		return
	}

	masterUser := instance.MasterUser()
	if masterUser == nil {
		err = errors.New("Failed to find master user")
		return
	}
	masterPassword, err := masterUser.Password(b.encryptionKey)
	if err != nil {
		return
	}

	sqlEngine, err = b.sqlProvider.GetSQLEngine(engine)
	if err != nil {
		return
	}

	err = sqlEngine.Open(dbAddress, dbPort, dbName, masterUser.Username, masterPassword, config.RequireNoVerify)
	if err != nil {
		return
	}
	return
}

func (b *RDSBroker) createDBCluster(instance *internaldb.DBInstance, servicePlan ServicePlan, provisionParameters ProvisionParameters, details brokerapi.ProvisionDetails) *awsrds.DBClusterDetails {
	dbClusterDetails := b.dbClusterFromPlan(servicePlan)
	dbClusterDetails.DatabaseName = instance.DBName
	dbClusterDetails.MasterUsername = instance.MasterUser().Username
	var err error
	dbClusterDetails.MasterUserPassword, err = instance.MasterUser().Password(b.encryptionKey)
	if err != nil {
		b.logger.Error("get-password", err)
		return nil
	}

	if provisionParameters.BackupRetentionPeriod > 0 {
		dbClusterDetails.BackupRetentionPeriod = provisionParameters.BackupRetentionPeriod
	}

	if provisionParameters.PreferredBackupWindow != "" {
		dbClusterDetails.PreferredBackupWindow = provisionParameters.PreferredBackupWindow
	}

	if provisionParameters.PreferredMaintenanceWindow != "" {
		dbClusterDetails.PreferredMaintenanceWindow = provisionParameters.PreferredMaintenanceWindow
	}

	dbClusterDetails.Tags = b.dbTags("Created", details.ServiceID, details.PlanID, details.OrganizationGUID, details.SpaceGUID)

	return dbClusterDetails
}

func (b *RDSBroker) modifyDBCluster(instance *internaldb.DBInstance, servicePlan ServicePlan, updateParameters UpdateParameters, details brokerapi.UpdateDetails) *awsrds.DBClusterDetails {
	dbClusterDetails := b.dbClusterFromPlan(servicePlan)

	if updateParameters.BackupRetentionPeriod > 0 {
		dbClusterDetails.BackupRetentionPeriod = updateParameters.BackupRetentionPeriod
	}

	if updateParameters.PreferredBackupWindow != "" {
		dbClusterDetails.PreferredBackupWindow = updateParameters.PreferredBackupWindow
	}

	if updateParameters.PreferredMaintenanceWindow != "" {
		dbClusterDetails.PreferredMaintenanceWindow = updateParameters.PreferredMaintenanceWindow
	}

	dbClusterDetails.Tags = b.dbTags("Updated", details.ServiceID, details.PlanID, "", "")

	return dbClusterDetails
}

func (b *RDSBroker) dbClusterFromPlan(servicePlan ServicePlan) *awsrds.DBClusterDetails {
	dbClusterDetails := &awsrds.DBClusterDetails{
		Engine: servicePlan.RDSProperties.Engine,
	}

	if servicePlan.RDSProperties.AvailabilityZone != "" {
		dbClusterDetails.AvailabilityZones = []string{servicePlan.RDSProperties.AvailabilityZone}
	}

	if servicePlan.RDSProperties.BackupRetentionPeriod > 0 {
		dbClusterDetails.BackupRetentionPeriod = servicePlan.RDSProperties.BackupRetentionPeriod
	}

	if servicePlan.RDSProperties.DBClusterParameterGroupName != "" {
		dbClusterDetails.DBClusterParameterGroupName = servicePlan.RDSProperties.DBClusterParameterGroupName
	}

	if servicePlan.RDSProperties.DBSubnetGroupName != "" {
		dbClusterDetails.DBSubnetGroupName = servicePlan.RDSProperties.DBSubnetGroupName
	}

	if servicePlan.RDSProperties.EngineVersion != "" {
		dbClusterDetails.EngineVersion = servicePlan.RDSProperties.EngineVersion
	}

	if servicePlan.RDSProperties.Port > 0 {
		dbClusterDetails.Port = servicePlan.RDSProperties.Port
	}

	if servicePlan.RDSProperties.PreferredBackupWindow != "" {
		dbClusterDetails.PreferredBackupWindow = servicePlan.RDSProperties.PreferredBackupWindow
	}

	if servicePlan.RDSProperties.PreferredMaintenanceWindow != "" {
		dbClusterDetails.PreferredMaintenanceWindow = servicePlan.RDSProperties.PreferredMaintenanceWindow
	}

	if len(servicePlan.RDSProperties.VpcSecurityGroupIds) > 0 {
		dbClusterDetails.VpcSecurityGroupIds = servicePlan.RDSProperties.VpcSecurityGroupIds
	}

	return dbClusterDetails
}

func (b *RDSBroker) createDBInstance(instance *internaldb.DBInstance, servicePlan ServicePlan, provisionParameters ProvisionParameters, details brokerapi.ProvisionDetails) *awsrds.DBInstanceDetails {
	dbInstanceDetails := b.dbInstanceFromPlan(servicePlan)

	if strings.ToLower(servicePlan.RDSProperties.Engine) == "aurora" {
		dbInstanceDetails.DBClusterIdentifier = b.dbClusterIdentifier(instance)
	} else {
		dbInstanceDetails.DBName = instance.DBName
		dbInstanceDetails.MasterUsername = instance.MasterUser().Username
		var err error
		dbInstanceDetails.MasterUserPassword, err = instance.MasterUser().Password(b.encryptionKey)
		if err != nil {
			b.logger.Error("get-password", err)
			return nil
		}

		if provisionParameters.BackupRetentionPeriod > 0 {
			dbInstanceDetails.BackupRetentionPeriod = provisionParameters.BackupRetentionPeriod
		}

		if provisionParameters.CharacterSetName != "" {
			dbInstanceDetails.CharacterSetName = provisionParameters.CharacterSetName
		}

		if provisionParameters.PreferredBackupWindow != "" {
			dbInstanceDetails.PreferredBackupWindow = provisionParameters.PreferredBackupWindow
		}
	}

	if provisionParameters.PreferredMaintenanceWindow != "" {
		dbInstanceDetails.PreferredMaintenanceWindow = provisionParameters.PreferredMaintenanceWindow
	}

	dbInstanceDetails.Tags = b.dbTags("Created", details.ServiceID, details.PlanID, details.OrganizationGUID, details.SpaceGUID)

	return dbInstanceDetails
}

func (b *RDSBroker) modifyDBInstance(instance *internaldb.DBInstance, servicePlan ServicePlan, updateParameters UpdateParameters, details brokerapi.UpdateDetails) *awsrds.DBInstanceDetails {
	dbInstanceDetails := b.dbInstanceFromPlan(servicePlan)

	if strings.ToLower(servicePlan.RDSProperties.Engine) != "aurora" {
		if updateParameters.BackupRetentionPeriod > 0 {
			dbInstanceDetails.BackupRetentionPeriod = updateParameters.BackupRetentionPeriod
		}

		if updateParameters.PreferredBackupWindow != "" {
			dbInstanceDetails.PreferredBackupWindow = updateParameters.PreferredBackupWindow
		}
	}

	if updateParameters.PreferredMaintenanceWindow != "" {
		dbInstanceDetails.PreferredMaintenanceWindow = updateParameters.PreferredMaintenanceWindow
	}

	dbInstanceDetails.Tags = b.dbTags("Updated", details.ServiceID, details.PlanID, "", "")

	return dbInstanceDetails
}

func (b *RDSBroker) dbInstanceFromPlan(servicePlan ServicePlan) *awsrds.DBInstanceDetails {
	dbInstanceDetails := &awsrds.DBInstanceDetails{
		DBInstanceClass: servicePlan.RDSProperties.DBInstanceClass,
		Engine:          servicePlan.RDSProperties.Engine,
	}

	dbInstanceDetails.AutoMinorVersionUpgrade = servicePlan.RDSProperties.AutoMinorVersionUpgrade

	if servicePlan.RDSProperties.AvailabilityZone != "" {
		dbInstanceDetails.AvailabilityZone = servicePlan.RDSProperties.AvailabilityZone
	}

	dbInstanceDetails.CopyTagsToSnapshot = servicePlan.RDSProperties.CopyTagsToSnapshot

	if servicePlan.RDSProperties.DBParameterGroupName != "" {
		dbInstanceDetails.DBParameterGroupName = servicePlan.RDSProperties.DBParameterGroupName
	}

	if servicePlan.RDSProperties.DBSubnetGroupName != "" {
		dbInstanceDetails.DBSubnetGroupName = servicePlan.RDSProperties.DBSubnetGroupName
	}

	if servicePlan.RDSProperties.EngineVersion != "" {
		dbInstanceDetails.EngineVersion = servicePlan.RDSProperties.EngineVersion
	}

	if servicePlan.RDSProperties.OptionGroupName != "" {
		dbInstanceDetails.OptionGroupName = servicePlan.RDSProperties.OptionGroupName
	}

	if servicePlan.RDSProperties.PreferredMaintenanceWindow != "" {
		dbInstanceDetails.PreferredMaintenanceWindow = servicePlan.RDSProperties.PreferredMaintenanceWindow
	}

	dbInstanceDetails.PubliclyAccessible = servicePlan.RDSProperties.PubliclyAccessible

	if strings.ToLower(servicePlan.RDSProperties.Engine) != "aurora" {
		if servicePlan.RDSProperties.AllocatedStorage > 0 {
			dbInstanceDetails.AllocatedStorage = servicePlan.RDSProperties.AllocatedStorage
		}

		if servicePlan.RDSProperties.BackupRetentionPeriod > 0 {
			dbInstanceDetails.BackupRetentionPeriod = servicePlan.RDSProperties.BackupRetentionPeriod
		}

		if servicePlan.RDSProperties.CharacterSetName != "" {
			dbInstanceDetails.CharacterSetName = servicePlan.RDSProperties.CharacterSetName
		}

		if len(servicePlan.RDSProperties.DBSecurityGroups) > 0 {
			dbInstanceDetails.DBSecurityGroups = servicePlan.RDSProperties.DBSecurityGroups
		}

		if servicePlan.RDSProperties.Iops > 0 {
			dbInstanceDetails.Iops = servicePlan.RDSProperties.Iops
		}

		if servicePlan.RDSProperties.KmsKeyID != "" {
			dbInstanceDetails.KmsKeyID = servicePlan.RDSProperties.KmsKeyID
		}

		if servicePlan.RDSProperties.LicenseModel != "" {
			dbInstanceDetails.LicenseModel = servicePlan.RDSProperties.LicenseModel
		}

		dbInstanceDetails.MultiAZ = servicePlan.RDSProperties.MultiAZ

		if servicePlan.RDSProperties.Port > 0 {
			dbInstanceDetails.Port = servicePlan.RDSProperties.Port
		}

		if servicePlan.RDSProperties.PreferredBackupWindow != "" {
			dbInstanceDetails.PreferredBackupWindow = servicePlan.RDSProperties.PreferredBackupWindow
		}

		dbInstanceDetails.StorageEncrypted = servicePlan.RDSProperties.StorageEncrypted

		if servicePlan.RDSProperties.StorageType != "" {
			dbInstanceDetails.StorageType = servicePlan.RDSProperties.StorageType
		}

		if len(servicePlan.RDSProperties.VpcSecurityGroupIds) > 0 {
			dbInstanceDetails.VpcSecurityGroupIds = servicePlan.RDSProperties.VpcSecurityGroupIds
		}
	}

	return dbInstanceDetails
}

func (b *RDSBroker) dbTags(action, serviceID, planID, organizationID, spaceID string) map[string]string {
	tags := make(map[string]string)

	tags["Owner"] = "Cloud Foundry"

	tags[action+" by"] = "AWS RDS Service Broker"

	tags[action+" at"] = time.Now().Format(time.RFC822Z)

	if serviceID != "" {
		tags["Service ID"] = serviceID
	}

	if planID != "" {
		tags["Plan ID"] = planID
	}

	if organizationID != "" {
		tags["Organization ID"] = organizationID
	}

	if spaceID != "" {
		tags["Space ID"] = spaceID
	}

	return tags
}

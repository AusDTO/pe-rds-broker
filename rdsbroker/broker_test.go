package rdsbroker_test

import (
	"errors"
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/rdsbroker"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/pivotal-cf/brokerapi"

	"github.com/AusDTO/pe-rds-broker/awsrds"
	rdsfake "github.com/AusDTO/pe-rds-broker/awsrds/fakes"
	sqlfake "github.com/AusDTO/pe-rds-broker/sqlengine/fakes"
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"os"
	"github.com/jinzhu/gorm"
	"github.com/AusDTO/pe-rds-broker/config"
)

var _ = Describe("RDS Broker", func() {
	var (
		rdsProperties1 RDSProperties
		rdsProperties2 RDSProperties
		plan1          ServicePlan
		plan2          ServicePlan
		service1       Service
		service2       Service
		catalog        Catalog

		configYml Config

		dbInstance *rdsfake.FakeDBInstance
		dbCluster  *rdsfake.FakeDBCluster

		sqlProvider    *sqlfake.FakeProvider
		sqlEngine      *sqlfake.FakeSQLEngine
		sharedPostgres *sqlfake.FakeSQLEngine
		sharedMysql    *sqlfake.FakeSQLEngine

		testSink *lagertest.TestSink
		logger   lager.Logger
		encryptionKey []byte
		internalDB *gorm.DB

		rdsBroker *RDSBroker

		allowUserProvisionParameters bool
		allowUserUpdateParameters    bool
		allowUserBindParameters      bool
		serviceBindable              bool
		planUpdateable               bool
		skipFinalSnapshot            bool

		instanceID           = "instance-id"
		bindingID            = "binding-id"
		dbInstanceIdentifier = "cf-instance-id"
		dbClusterIdentifier  = "cf-instance-id"
		dbName               = "cf_instance_id"
	)

	BeforeEach(func() {
		allowUserProvisionParameters = true
		allowUserUpdateParameters = true
		allowUserBindParameters = true
		serviceBindable = true
		planUpdateable = true
		skipFinalSnapshot = true

		dbInstance = &rdsfake.FakeDBInstance{}
		dbCluster = &rdsfake.FakeDBCluster{}

		sqlProvider = &sqlfake.FakeProvider{}
		sqlEngine = &sqlfake.FakeSQLEngine{}
		sharedPostgres = &sqlfake.FakeSQLEngine{}
		sharedMysql = &sqlfake.FakeSQLEngine{}
		sqlProvider.GetSQLEngineSQLEngine = sqlEngine
		encryptionKey = make([]byte, 32)

		rdsProperties1 = RDSProperties{
			DBInstanceClass:   "db.m1.test",
			Engine:            "test-engine-1",
			EngineVersion:     "1.2.3",
			AllocatedStorage:  100,
			SkipFinalSnapshot: skipFinalSnapshot,
		}

		rdsProperties2 = RDSProperties{
			DBInstanceClass:   "db.m2.test",
			Engine:            "test-engine-2",
			EngineVersion:     "4.5.6",
			AllocatedStorage:  200,
			SkipFinalSnapshot: skipFinalSnapshot,
		}
		// So I tried deleting all entries in an AfterEach block but it takes
		// the same amount of time as this and you have to manually add each new
		// model to the list. So rm-ing the database it is.
		os.Remove("/tmp/test.sqlite3")
		var err error
		internalDB, err = internaldb.DBInit(&config.DBConfig{DBType: "sqlite3", DBName: "/tmp/test.sqlite3"}, logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(internalDB).NotTo(BeNil())
	})

	JustBeforeEach(func() {
		plan1 = ServicePlan{
			ID:            "Plan-1",
			Name:          "Plan 1",
			Description:   "This is the Plan 1",
			RDSProperties: rdsProperties1,
		}
		plan2 = ServicePlan{
			ID:            "Plan-2",
			Name:          "Plan 2",
			Description:   "This is the Plan 2",
			RDSProperties: rdsProperties2,
		}

		service1 = Service{
			ID:             "Service-1",
			Name:           "Service 1",
			Description:    "This is the Service 1",
			Bindable:       serviceBindable,
			PlanUpdateable: planUpdateable,
			Plans:          []ServicePlan{plan1},
		}
		service2 = Service{
			ID:             "Service-2",
			Name:           "Service 2",
			Description:    "This is the Service 2",
			Bindable:       serviceBindable,
			PlanUpdateable: planUpdateable,
			Plans:          []ServicePlan{plan2},
		}

		catalog = Catalog{
			Services: []Service{service1, service2},
		}

		configYml = Config{
			Region:                       "rds-region",
			DBPrefix:                     "cf",
			AllowUserProvisionParameters: allowUserProvisionParameters,
			AllowUserUpdateParameters:    allowUserUpdateParameters,
			AllowUserBindParameters:      allowUserBindParameters,
			Catalog:                      catalog,
		}

		logger = lager.NewLogger("rdsbroker_test")
		testSink = lagertest.NewTestSink()
		logger.RegisterSink(testSink)

		rdsBroker = New(configYml, dbInstance, dbCluster, sqlProvider, logger, internalDB, sharedPostgres, sharedMysql, encryptionKey)
	})

	var MakeInstance = func() *internaldb.DBInstance {
		instance, err := internaldb.NewInstance(service1.ID, plan1.ID, instanceID, configYml.DBPrefix, encryptionKey)
		Expect(err).NotTo(HaveOccurred())
		err = internalDB.Save(instance).Error
		Expect(err).NotTo(HaveOccurred())
		return instance
	}

	var _ = Describe("Services", func() {
		var (
			properCatalogResponse []brokerapi.Service
		)

		BeforeEach(func() {
			properCatalogResponse = []brokerapi.Service{
				brokerapi.Service{
					ID:             "Service-1",
					Name:           "Service 1",
					Description:    "This is the Service 1",
					Bindable:       serviceBindable,
					PlanUpdatable: planUpdateable,
					Plans: []brokerapi.ServicePlan{
						brokerapi.ServicePlan{
							ID:          "Plan-1",
							Name:        "Plan 1",
							Description: "This is the Plan 1",
						},
					},
				},
				brokerapi.Service{
					ID:             "Service-2",
					Name:           "Service 2",
					Description:    "This is the Service 2",
					Bindable:       serviceBindable,
					PlanUpdatable: planUpdateable,
					Plans: []brokerapi.ServicePlan{
						brokerapi.ServicePlan{
							ID:          "Plan-2",
							Name:        "Plan 2",
							Description: "This is the Plan 2",
						},
					},
				},
			}
		})

		It("returns the proper CatalogResponse", func() {
			brokerCatalog := rdsBroker.Services(context.Background())
			Expect(brokerCatalog).To(Equal(properCatalogResponse))
		})

	})

	var _ = Describe("Provision", func() {
		var (
			provisionDetails  brokerapi.ProvisionDetails
			acceptsIncomplete bool

			properProvisionedServiceSpec brokerapi.ProvisionedServiceSpec
		)

		BeforeEach(func() {
			provisionDetails = brokerapi.ProvisionDetails{
				OrganizationGUID: "organization-id",
				PlanID:           "Plan-1",
				ServiceID:        "Service-1",
				SpaceGUID:        "space-id",
				RawParameters:    json.RawMessage(""),
			}
			acceptsIncomplete = true

			properProvisionedServiceSpec = brokerapi.ProvisionedServiceSpec{
				IsAsync: true,
			}
		})

		Provision := func() (brokerapi.ProvisionedServiceSpec, error) {
			return rdsBroker.Provision(context.Background(), instanceID, provisionDetails, acceptsIncomplete)
		}

		It("returns the proper response", func() {
			provisionedServiceSpec, err := Provision()
			Expect(provisionedServiceSpec).To(Equal(properProvisionedServiceSpec))
			Expect(err).ToNot(HaveOccurred())
		})

		It("makes the proper calls", func() {
			_, err := Provision()
			Expect(dbInstance.CreateCalled).To(BeTrue())
			Expect(dbInstance.CreateID).To(Equal(dbInstanceIdentifier))
			Expect(dbInstance.CreateDBInstanceDetails.DBInstanceClass).To(Equal("db.m1.test"))
			Expect(dbInstance.CreateDBInstanceDetails.Engine).To(Equal("test-engine-1"))
			Expect(dbInstance.CreateDBInstanceDetails.DBName).To(Equal(dbName))
			Expect(dbInstance.CreateDBInstanceDetails.MasterUsername).ToNot(BeEmpty())
			Expect(dbInstance.CreateDBInstanceDetails.MasterUserPassword).ToNot(BeEmpty())
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Owner"]).To(Equal("Cloud Foundry"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Created by"]).To(Equal("AWS RDS Service Broker"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags).To(HaveKey("Created at"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Service ID"]).To(Equal("Service-1"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Plan ID"]).To(Equal("Plan-1"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Organization ID"]).To(Equal("organization-id"))
			Expect(dbInstance.CreateDBInstanceDetails.Tags["Space ID"]).To(Equal("space-id"))
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when has AllocatedStorage", func() {
			BeforeEach(func() {
				rdsProperties1.AllocatedStorage = int64(100)
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.AllocatedStorage).To(Equal(int64(100)))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.AllocatedStorage).To(Equal(int64(0)))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has AutoMinorVersionUpgrade", func() {
			BeforeEach(func() {
				rdsProperties1.AutoMinorVersionUpgrade = true
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.AutoMinorVersionUpgrade).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when has AvailabilityZone", func() {
			BeforeEach(func() {
				rdsProperties1.AvailabilityZone = "test-az"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.AvailabilityZone).To(Equal("test-az"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.AvailabilityZone).To(Equal("test-az"))
					Expect(dbCluster.CreateDBClusterDetails.AvailabilityZones).To(Equal([]string{"test-az"}))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has BackupRetentionPeriod", func() {
			BeforeEach(func() {
				rdsProperties1.BackupRetentionPeriod = int64(7)
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(7)))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.BackupRetentionPeriod).To(Equal(int64(7)))
					Expect(dbInstance.CreateDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(0)))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("but has BackupRetentionPeriod Parameter", func() {
				BeforeEach(func() {
					provisionDetails.RawParameters = json.RawMessage("{\"backup_retention_period\":12}")
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(12)))
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Provision()
						Expect(dbCluster.CreateDBClusterDetails.BackupRetentionPeriod).To(Equal(int64(12)))
						Expect(dbInstance.CreateDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(0)))
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})

		Context("when has CharacterSetName", func() {
			BeforeEach(func() {
				rdsProperties1.CharacterSetName = "test-characterset-name"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.CharacterSetName).To(Equal("test-characterset-name"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.CharacterSetName).To(Equal(""))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("but has CharacterSetName Parameter", func() {
				BeforeEach(func() {
					provisionDetails.RawParameters = json.RawMessage("{\"character_set_name\": \"test-characterset-name-parameter\"}")
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.CharacterSetName).To(Equal("test-characterset-name-parameter"))
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Provision()
						Expect(dbInstance.CreateDBInstanceDetails.CharacterSetName).To(Equal(""))
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})

		Context("when has CopyTagsToSnapshot", func() {
			BeforeEach(func() {
				rdsProperties1.CopyTagsToSnapshot = true
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.CopyTagsToSnapshot).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when has DBParameterGroupName", func() {
			BeforeEach(func() {
				rdsProperties1.DBParameterGroupName = "test-db-parameter-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.DBParameterGroupName).To(Equal("test-db-parameter-group-name"))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when has DBSecurityGroups", func() {
			BeforeEach(func() {
				rdsProperties1.DBSecurityGroups = []string{"test-db-security-group"}
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.DBSecurityGroups).To(Equal([]string{"test-db-security-group"}))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.DBSecurityGroups).To(BeNil())
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has DBSubnetGroupName", func() {
			BeforeEach(func() {
				rdsProperties1.DBSubnetGroupName = "test-db-subnet-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.DBSubnetGroupName).To(Equal("test-db-subnet-group-name"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.DBSubnetGroupName).To(Equal("test-db-subnet-group-name"))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has EngineVersion", func() {
			BeforeEach(func() {
				rdsProperties1.EngineVersion = "1.2.3"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.EngineVersion).To(Equal("1.2.3"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.EngineVersion).To(Equal("1.2.3"))
					Expect(dbCluster.CreateDBClusterDetails.EngineVersion).To(Equal("1.2.3"))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has Iops", func() {
			BeforeEach(func() {
				rdsProperties1.Iops = int64(1000)
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.Iops).To(Equal(int64(1000)))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.Iops).To(Equal(int64(0)))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has KmsKeyID", func() {
			BeforeEach(func() {
				rdsProperties1.KmsKeyID = "test-kms-key-id"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.KmsKeyID).To(Equal("test-kms-key-id"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.KmsKeyID).To(Equal(""))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has LicenseModel", func() {
			BeforeEach(func() {
				rdsProperties1.LicenseModel = "test-license-model"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.LicenseModel).To(Equal("test-license-model"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.LicenseModel).To(Equal(""))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has MultiAZ", func() {
			BeforeEach(func() {
				rdsProperties1.MultiAZ = true
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.MultiAZ).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.MultiAZ).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has OptionGroupName", func() {
			BeforeEach(func() {
				rdsProperties1.OptionGroupName = "test-option-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.OptionGroupName).To(Equal("test-option-group-name"))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when has Port", func() {
			BeforeEach(func() {
				rdsProperties1.Port = int64(3306)
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.Port).To(Equal(int64(3306)))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.Port).To(Equal(int64(3306)))
					Expect(dbInstance.CreateDBInstanceDetails.Port).To(Equal(int64(0)))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has PreferredBackupWindow", func() {
			BeforeEach(func() {
				rdsProperties1.PreferredBackupWindow = "test-preferred-backup-window"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window"))
					Expect(dbInstance.CreateDBInstanceDetails.PreferredBackupWindow).To(Equal(""))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("but has PreferredBackupWindow Parameter", func() {
				BeforeEach(func() {
					provisionDetails.RawParameters = json.RawMessage("{\"preferred_backup_window\": \"test-preferred-backup-window-parameter\"}")
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window-parameter"))
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Provision()
						Expect(dbCluster.CreateDBClusterDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window-parameter"))
						Expect(dbInstance.CreateDBInstanceDetails.PreferredBackupWindow).To(Equal(""))
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})

		Context("when has PreferredMaintenanceWindow", func() {
			BeforeEach(func() {
				rdsProperties1.PreferredMaintenanceWindow = "test-preferred-maintenance-window"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window"))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("but has PreferredMaintenanceWindow Parameter", func() {
				BeforeEach(func() {
					provisionDetails.RawParameters = json.RawMessage("{\"preferred_maintenance_window\": \"test-preferred-maintenance-window-parameter\"}")
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window-parameter"))
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Provision()
						Expect(dbCluster.CreateDBClusterDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window-parameter"))
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})

		Context("when has PubliclyAccessible", func() {
			BeforeEach(func() {
				rdsProperties1.PubliclyAccessible = true
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.PubliclyAccessible).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when has StorageEncrypted", func() {
			BeforeEach(func() {
				rdsProperties1.StorageEncrypted = true
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.StorageEncrypted).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.StorageEncrypted).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has StorageType", func() {
			BeforeEach(func() {
				rdsProperties1.StorageType = "test-storage-type"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.StorageType).To(Equal("test-storage-type"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateDBInstanceDetails.StorageType).To(Equal(""))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when has VpcSecurityGroupIds", func() {
			BeforeEach(func() {
				rdsProperties1.VpcSecurityGroupIds = []string{"test-vpc-security-group-ids"}
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbInstance.CreateDBInstanceDetails.VpcSecurityGroupIds).To(Equal([]string{"test-vpc-security-group-ids"}))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.VpcSecurityGroupIds).To(Equal([]string{"test-vpc-security-group-ids"}))
					Expect(dbInstance.CreateDBInstanceDetails.VpcSecurityGroupIds).To(BeNil())
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when request does not accept incomplete", func() {
			BeforeEach(func() {
				acceptsIncomplete = false
			})

			It("returns the proper error", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(brokerapi.ErrAsyncRequired))
			})
		})

		Context("when Parameters are not valid", func() {
			BeforeEach(func() {
				provisionDetails.RawParameters = json.RawMessage("{\"backup_retention_period\": \"invalid\"}")
			})

			It("returns the proper error", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("json: cannot unmarshal string into Go value of type int64"))
			})

			Context("and user provision parameters are not allowed", func() {
				BeforeEach(func() {
					allowUserProvisionParameters = false
				})

				It("does not return an error", func() {
					_, err := Provision()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when Service Plan is not found", func() {
			BeforeEach(func() {
				provisionDetails.PlanID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service Plan 'unknown' not found"))
			})
		})

		Context("when creating the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.CreateError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})
		})

		Context("when instance id already exists", func() {
			BeforeEach(func() {
				MakeInstance()
			})

			It("returns the proper error", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Instance already exists"))
			})

			It("Doesn't create the database", func() {
				_, err := Provision()
				Expect(err).To(HaveOccurred())
				Expect(dbInstance.CreateCalled).To(BeFalse())
			})
		})

		Context("when shared instance", func() {
			BeforeEach(func() {
				rdsProperties1.Shared = true
				properProvisionedServiceSpec = brokerapi.ProvisionedServiceSpec{
					IsAsync: false,
				}
			})

			Context("with postgres", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "postgres"
				})

				It("returns the proper response", func() {
					provisionedServiceSpec, err := Provision()
					Expect(err).ToNot(HaveOccurred())
					Expect(provisionedServiceSpec).To(Equal(properProvisionedServiceSpec))
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateCalled).To(BeFalse())
					Expect(sharedPostgres.CreateDBCalled).To(BeTrue())
					Expect(sharedPostgres.CreateDBDBName).To(Equal(dbName))
					Expect(sharedMysql.CreateDBCalled).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("with mysql", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "mysql"
				})

				It("returns the proper response", func() {
					provisionedServiceSpec, err := Provision()
					Expect(err).ToNot(HaveOccurred())
					Expect(provisionedServiceSpec).To(Equal(properProvisionedServiceSpec))
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbInstance.CreateCalled).To(BeFalse())
					Expect(sharedPostgres.CreateDBCalled).To(BeFalse())
					Expect(sharedMysql.CreateDBCalled).To(BeTrue())
					Expect(sharedMysql.CreateDBDBName).To(Equal(dbName))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when Engine is Aurora", func() {
			BeforeEach(func() {
				rdsProperties1.Engine = "aurora"
			})

			It("makes the proper calls", func() {
				_, err := Provision()
				Expect(dbCluster.CreateCalled).To(BeTrue())
				Expect(dbCluster.CreateID).To(Equal(dbClusterIdentifier))
				Expect(dbCluster.CreateDBClusterDetails.Engine).To(Equal("aurora"))
				Expect(dbCluster.CreateDBClusterDetails.DatabaseName).To(Equal(dbName))
				Expect(dbCluster.CreateDBClusterDetails.MasterUsername).ToNot(BeEmpty())
				Expect(dbCluster.CreateDBClusterDetails.MasterUserPassword).ToNot(BeEmpty())
				Expect(dbCluster.CreateDBClusterDetails.Tags["Owner"]).To(Equal("Cloud Foundry"))
				Expect(dbCluster.CreateDBClusterDetails.Tags["Created by"]).To(Equal("AWS RDS Service Broker"))
				Expect(dbCluster.CreateDBClusterDetails.Tags).To(HaveKey("Created at"))
				Expect(dbCluster.CreateDBClusterDetails.Tags["Service ID"]).To(Equal("Service-1"))
				Expect(dbCluster.CreateDBClusterDetails.Tags["Plan ID"]).To(Equal("Plan-1"))
				Expect(dbCluster.CreateDBClusterDetails.Tags["Organization ID"]).To(Equal("organization-id"))
				Expect(dbCluster.CreateDBClusterDetails.Tags["Space ID"]).To(Equal("space-id"))
				Expect(dbInstance.CreateDBInstanceDetails.DBClusterIdentifier).To(Equal(dbClusterIdentifier))
				Expect(dbCluster.DeleteCalled).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when has DBClusterParameterGroupName", func() {
				BeforeEach(func() {
					rdsProperties1.DBClusterParameterGroupName = "test-db-cluster-parameter-group-name"
				})

				It("makes the proper calls", func() {
					_, err := Provision()
					Expect(dbCluster.CreateDBClusterDetails.DBClusterParameterGroupName).To(Equal("test-db-cluster-parameter-group-name"))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when creating the DB Instance fails", func() {
				BeforeEach(func() {
					dbInstance.CreateError = errors.New("operation failed")
				})

				It("deletes the DB Cluster", func() {
					_, err := Provision()
					Expect(err).To(HaveOccurred())
					Expect(dbCluster.DeleteCalled).To(BeTrue())
					Expect(dbCluster.DeleteID).To(Equal(dbClusterIdentifier))
				})
			})
		})
	})

	var _ = Describe("Update", func() {
		var (
			updateDetails     brokerapi.UpdateDetails
			acceptsIncomplete bool
		)

		BeforeEach(func() {
			updateDetails = brokerapi.UpdateDetails{
				ServiceID:  "Service-2",
				PlanID:     "Plan-2",
				RawParameters: json.RawMessage(""),
				PreviousValues: brokerapi.PreviousValues{
					PlanID:         "Plan-1",
					ServiceID:      "Service-1",
					OrgID:          "organization-id",
					SpaceID:        "space-id",
				},
			}
			acceptsIncomplete = true
			MakeInstance()
		})

		Update := func() (brokerapi.UpdateServiceSpec, error) {
			return rdsBroker.Update(context.Background(), instanceID, updateDetails, acceptsIncomplete)
		}

		It("returns the proper response", func() {
			updateSpec, err := Update()
			Expect(err).ToNot(HaveOccurred())
			Expect(updateSpec.IsAsync).To(BeTrue())
		})

		It("makes the proper calls", func() {
			_, err := Update()
			Expect(err).ToNot(HaveOccurred())
			Expect(dbInstance.ModifyCalled).To(BeTrue())
			Expect(dbInstance.ModifyID).To(Equal(dbInstanceIdentifier))
			Expect(dbInstance.ModifyDBInstanceDetails.DBInstanceClass).To(Equal("db.m2.test"))
			Expect(dbInstance.ModifyDBInstanceDetails.Engine).To(Equal("test-engine-2"))
			Expect(dbInstance.ModifyDBInstanceDetails.Tags["Owner"]).To(Equal("Cloud Foundry"))
			Expect(dbInstance.ModifyDBInstanceDetails.Tags["Updated by"]).To(Equal("AWS RDS Service Broker"))
			Expect(dbInstance.ModifyDBInstanceDetails.Tags).To(HaveKey("Updated at"))
			Expect(dbInstance.ModifyDBInstanceDetails.Tags["Service ID"]).To(Equal("Service-2"))
			Expect(dbInstance.ModifyDBInstanceDetails.Tags["Plan ID"]).To(Equal("Plan-2"))
		})

		Context("when has AllocatedStorage", func() {
			BeforeEach(func() {
				rdsProperties2.AllocatedStorage = int64(100)
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.AllocatedStorage).To(Equal(int64(100)))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.AllocatedStorage).To(Equal(int64(0)))
				})
			})
		})

		Context("when has AutoMinorVersionUpgrade", func() {
			BeforeEach(func() {
				rdsProperties2.AutoMinorVersionUpgrade = true
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.AutoMinorVersionUpgrade).To(BeTrue())
			})
		})

		Context("when has AvailabilityZone", func() {
			BeforeEach(func() {
				rdsProperties2.AvailabilityZone = "test-az"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.AvailabilityZone).To(Equal("test-az"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.AvailabilityZone).To(Equal("test-az"))
					Expect(dbCluster.ModifyDBClusterDetails.AvailabilityZones).To(Equal([]string{"test-az"}))
				})
			})
		})

		Context("when has BackupRetentionPeriod", func() {
			BeforeEach(func() {
				rdsProperties2.BackupRetentionPeriod = int64(7)
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(7)))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.BackupRetentionPeriod).To(Equal(int64(7)))
					Expect(dbInstance.ModifyDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(0)))
				})
			})

			Context("but has BackupRetentionPeriod Parameter", func() {
				BeforeEach(func() {
					updateDetails.RawParameters = json.RawMessage("{\"backup_retention_period\": 12}")
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(12)))
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties2.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Update()
						Expect(err).ToNot(HaveOccurred())
						Expect(dbCluster.ModifyDBClusterDetails.BackupRetentionPeriod).To(Equal(int64(12)))
						Expect(dbInstance.ModifyDBInstanceDetails.BackupRetentionPeriod).To(Equal(int64(0)))
					})
				})
			})
		})

		Context("when has CharacterSetName", func() {
			BeforeEach(func() {
				rdsProperties2.CharacterSetName = "test-characterset-name"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.CharacterSetName).To(Equal("test-characterset-name"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.CharacterSetName).To(Equal(""))
				})
			})
		})

		Context("when has CopyTagsToSnapshot", func() {
			BeforeEach(func() {
				rdsProperties2.CopyTagsToSnapshot = true
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.CopyTagsToSnapshot).To(BeTrue())
			})
		})

		Context("when has DBParameterGroupName", func() {
			BeforeEach(func() {
				rdsProperties2.DBParameterGroupName = "test-db-parameter-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.DBParameterGroupName).To(Equal("test-db-parameter-group-name"))
			})
		})

		Context("when has DBSecurityGroups", func() {
			BeforeEach(func() {
				rdsProperties2.DBSecurityGroups = []string{"test-db-security-group"}
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.DBSecurityGroups).To(Equal([]string{"test-db-security-group"}))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.DBSecurityGroups).To(BeNil())
				})
			})
		})

		Context("when has DBSubnetGroupName", func() {
			BeforeEach(func() {
				rdsProperties2.DBSubnetGroupName = "test-db-subnet-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.DBSubnetGroupName).To(Equal("test-db-subnet-group-name"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.DBSubnetGroupName).To(Equal("test-db-subnet-group-name"))
				})
			})
		})

		Context("when has EngineVersion", func() {
			BeforeEach(func() {
				rdsProperties2.EngineVersion = "1.2.3"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.EngineVersion).To(Equal("1.2.3"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.EngineVersion).To(Equal("1.2.3"))
					Expect(dbCluster.ModifyDBClusterDetails.EngineVersion).To(Equal("1.2.3"))
				})
			})
		})

		Context("when has Iops", func() {
			BeforeEach(func() {
				rdsProperties2.Iops = int64(1000)
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.Iops).To(Equal(int64(1000)))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.Iops).To(Equal(int64(0)))
				})
			})
		})

		Context("when has KmsKeyID", func() {
			BeforeEach(func() {
				rdsProperties2.KmsKeyID = "test-kms-key-id"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.KmsKeyID).To(Equal("test-kms-key-id"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.KmsKeyID).To(Equal(""))
				})
			})
		})

		Context("when has LicenseModel", func() {
			BeforeEach(func() {
				rdsProperties2.LicenseModel = "test-license-model"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.LicenseModel).To(Equal("test-license-model"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.LicenseModel).To(Equal(""))
				})
			})
		})

		Context("when has MultiAZ", func() {
			BeforeEach(func() {
				rdsProperties2.MultiAZ = true
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.MultiAZ).To(BeTrue())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.MultiAZ).To(BeFalse())
				})
			})
		})

		Context("when has OptionGroupName", func() {
			BeforeEach(func() {
				rdsProperties2.OptionGroupName = "test-option-group-name"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.OptionGroupName).To(Equal("test-option-group-name"))
			})
		})

		Context("when has Port", func() {
			BeforeEach(func() {
				rdsProperties2.Port = int64(3306)
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.Port).To(Equal(int64(3306)))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.Port).To(Equal(int64(3306)))
					Expect(dbInstance.ModifyDBInstanceDetails.Port).To(Equal(int64(0)))
				})
			})
		})

		Context("when has PreferredBackupWindow", func() {
			BeforeEach(func() {
				rdsProperties2.PreferredBackupWindow = "test-preferred-backup-window"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window"))
					Expect(dbInstance.ModifyDBInstanceDetails.PreferredBackupWindow).To(Equal(""))
				})
			})

			Context("but has PreferredBackupWindow Parameter", func() {
				BeforeEach(func() {
					updateDetails.RawParameters = json.RawMessage("{\"preferred_backup_window\": \"test-preferred-backup-window-parameter\"}")
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window-parameter"))
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties2.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Update()
						Expect(err).ToNot(HaveOccurred())
						Expect(dbCluster.ModifyDBClusterDetails.PreferredBackupWindow).To(Equal("test-preferred-backup-window-parameter"))
						Expect(dbInstance.ModifyDBInstanceDetails.PreferredBackupWindow).To(Equal(""))
					})
				})
			})
		})

		Context("when has PreferredMaintenanceWindow", func() {
			BeforeEach(func() {
				rdsProperties2.PreferredMaintenanceWindow = "test-preferred-maintenance-window"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window"))
				})
			})

			Context("but has PreferredMaintenanceWindow Parameter", func() {
				BeforeEach(func() {
					updateDetails.RawParameters = json.RawMessage("{\"preferred_maintenance_window\": \"test-preferred-maintenance-window-parameter\"}")
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window-parameter"))
				})

				Context("when Engine is Aurora", func() {
					BeforeEach(func() {
						rdsProperties2.Engine = "aurora"
					})

					It("makes the proper calls", func() {
						_, err := Update()
						Expect(err).ToNot(HaveOccurred())
						Expect(dbCluster.ModifyDBClusterDetails.PreferredMaintenanceWindow).To(Equal("test-preferred-maintenance-window-parameter"))
					})
				})
			})
		})

		Context("when has PubliclyAccessible", func() {
			BeforeEach(func() {
				rdsProperties2.PubliclyAccessible = true
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.PubliclyAccessible).To(BeTrue())
			})
		})

		Context("when has StorageEncrypted", func() {
			BeforeEach(func() {
				rdsProperties2.StorageEncrypted = true
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.StorageEncrypted).To(BeTrue())
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.StorageEncrypted).To(BeFalse())
				})
			})
		})

		Context("when has StorageType", func() {
			BeforeEach(func() {
				rdsProperties2.StorageType = "test-storage-type"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.StorageType).To(Equal("test-storage-type"))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbInstance.ModifyDBInstanceDetails.StorageType).To(Equal(""))
				})
			})
		})

		Context("when has VpcSecurityGroupIds", func() {
			BeforeEach(func() {
				rdsProperties2.VpcSecurityGroupIds = []string{"test-vpc-security-group-ids"}
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.ModifyDBInstanceDetails.VpcSecurityGroupIds).To(Equal([]string{"test-vpc-security-group-ids"}))
			})

			Context("when Engine is Aurora", func() {
				BeforeEach(func() {
					rdsProperties2.Engine = "aurora"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.VpcSecurityGroupIds).To(Equal([]string{"test-vpc-security-group-ids"}))
					Expect(dbInstance.ModifyDBInstanceDetails.VpcSecurityGroupIds).To(BeNil())
				})
			})
		})

		Context("when request does not accept incomplete", func() {
			BeforeEach(func() {
				acceptsIncomplete = false
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(brokerapi.ErrAsyncRequired))
			})
		})

		Context("when Parameters are not valid", func() {
			BeforeEach(func() {
				updateDetails.RawParameters = json.RawMessage("{\"backup_retention_period\": \"invalid\"}")
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("json: cannot unmarshal string into Go value of type int64"))
			})

			Context("and user update parameters are not allowed", func() {
				BeforeEach(func() {
					allowUserUpdateParameters = false
				})

				It("does not return an error", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when Service is not found", func() {
			BeforeEach(func() {
				updateDetails.ServiceID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service 'unknown' not found"))
			})
		})

		Context("when Plans is not updateable", func() {
			BeforeEach(func() {
				planUpdateable = false
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(brokerapi.ErrPlanChangeNotSupported))
			})
		})

		Context("when Service Plan is not found", func() {
			BeforeEach(func() {
				updateDetails.PlanID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service Plan 'unknown' not found"))
			})
		})

		Context("when modifying the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.ModifyError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				_, err := Update()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})

			Context("when the DB Instance does not exists", func() {
				BeforeEach(func() {
					dbInstance.ModifyError = awsrds.ErrDBInstanceDoesNotExist
				})

				It("returns the proper error", func() {
					_, err := Update()
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
				})
			})
		})

		Context("when Engine is Aurora", func() {
			BeforeEach(func() {
				rdsProperties2.Engine = "aurora"
			})

			It("makes the proper calls", func() {
				_, err := Update()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbCluster.ModifyCalled).To(BeTrue())
				Expect(dbCluster.ModifyID).To(Equal(dbClusterIdentifier))
				Expect(dbCluster.ModifyDBClusterDetails.Engine).To(Equal("aurora"))
				Expect(dbCluster.ModifyDBClusterDetails.Tags["Owner"]).To(Equal("Cloud Foundry"))
				Expect(dbCluster.ModifyDBClusterDetails.Tags["Updated by"]).To(Equal("AWS RDS Service Broker"))
				Expect(dbCluster.ModifyDBClusterDetails.Tags).To(HaveKey("Updated at"))
				Expect(dbCluster.ModifyDBClusterDetails.Tags["Service ID"]).To(Equal("Service-2"))
				Expect(dbCluster.ModifyDBClusterDetails.Tags["Plan ID"]).To(Equal("Plan-2"))
			})

			Context("when has DBClusterParameterGroupName", func() {
				BeforeEach(func() {
					rdsProperties2.DBClusterParameterGroupName = "test-db-cluster-parameter-group-name"
				})

				It("makes the proper calls", func() {
					_, err := Update()
					Expect(err).ToNot(HaveOccurred())
					Expect(dbCluster.ModifyDBClusterDetails.DBClusterParameterGroupName).To(Equal("test-db-cluster-parameter-group-name"))
				})
			})
		})
	})

	var _ = Describe("Deprovision", func() {
		var (
			deprovisionDetails brokerapi.DeprovisionDetails
			acceptsIncomplete  bool
		)

		BeforeEach(func() {
			deprovisionDetails = brokerapi.DeprovisionDetails{
				ServiceID: "Service-1",
				PlanID:    "Plan-1",
			}
			acceptsIncomplete = true
			MakeInstance()
		})

		Deprovision := func() (brokerapi.DeprovisionServiceSpec, error) {
			return rdsBroker.Deprovision(context.Background(), instanceID, deprovisionDetails, acceptsIncomplete)
		}

		It("returns the proper response", func() {
			deprovisionSpec, err := Deprovision()
			Expect(deprovisionSpec.IsAsync).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("makes the proper calls", func() {
			_, err := Deprovision()
			Expect(dbInstance.DeleteCalled).To(BeTrue())
			Expect(dbInstance.DeleteID).To(Equal(dbInstanceIdentifier))
			Expect(dbInstance.DeleteSkipFinalSnapshot).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It ("doesn't delete the internaldb instance", func() {
			_, err := Deprovision()
			Expect(err).ToNot(HaveOccurred())
			Expect(internaldb.FindInstance(internalDB, instanceID)).NotTo(BeNil())
		})

		Context("when it does not skip final snaphot", func() {
			BeforeEach(func() {
				rdsProperties1.SkipFinalSnapshot = false
			})

			It("makes the proper calls", func() {
				_, err := Deprovision()
				Expect(dbInstance.DeleteCalled).To(BeTrue())
				Expect(dbInstance.DeleteID).To(Equal(dbInstanceIdentifier))
				Expect(dbInstance.DeleteSkipFinalSnapshot).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when request does not accept incomplete", func() {
			BeforeEach(func() {
				acceptsIncomplete = false
			})

			It("returns the proper error", func() {
				_, err := Deprovision()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(brokerapi.ErrAsyncRequired))
			})
		})

		Context("when Service Plan is not found", func() {
			BeforeEach(func() {
				deprovisionDetails.PlanID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Deprovision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service Plan 'unknown' not found"))
			})
		})

		Context("when deleting the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.DeleteError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				_, err := Deprovision()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})

			Context("when the DB instance does not exists", func() {
				BeforeEach(func() {
					dbInstance.DeleteError = awsrds.ErrDBInstanceDoesNotExist
				})

				It("returns the proper error", func() {
					_, err := Deprovision()
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
				})
			})
		})

		Context("when shared instance", func() {
			BeforeEach(func() {
				rdsProperties1.Shared = true
			})

			Context("with postgres", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "postgres"
				})

				It("returns the proper response", func() {
					deprovisionSpec, err := Deprovision()
					Expect(err).ToNot(HaveOccurred())
					Expect(deprovisionSpec.IsAsync).To(BeFalse())
				})

				It("makes the proper calls", func() {
					_, err := Deprovision()
					Expect(dbInstance.DeleteCalled).To(BeFalse())
					Expect(sharedPostgres.DropDBCalled).To(BeTrue())
					Expect(sharedPostgres.DropDBDBName).To(Equal(dbName))
					Expect(sharedMysql.DropDBCalled).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})

				It ("deletes the internaldb instance", func() {
					_, err := Deprovision()
					Expect(err).ToNot(HaveOccurred())
					Expect(internaldb.FindInstance(internalDB, instanceID)).To(BeNil())
				})
			})

			Context("with mysql", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "mysql"
				})

				It("returns the proper response", func() {
					deprovisionSpec, err := Deprovision()
					Expect(err).ToNot(HaveOccurred())
					Expect(deprovisionSpec.IsAsync).To(BeFalse())
				})

				It("makes the proper calls", func() {
					_, err := Deprovision()
					Expect(dbInstance.DeleteCalled).To(BeFalse())
					Expect(sharedPostgres.DropDBCalled).To(BeFalse())
					Expect(sharedMysql.DropDBCalled).To(BeTrue())
					Expect(sharedMysql.DropDBDBName).To(Equal(dbName))
					Expect(err).ToNot(HaveOccurred())
				})

				It ("deletes the internaldb instance", func() {
					_, err := Deprovision()
					Expect(err).ToNot(HaveOccurred())
					Expect(internaldb.FindInstance(internalDB, instanceID)).To(BeNil())
				})
			})
		})

		Context("when Engine is Aurora", func() {
			BeforeEach(func() {
				rdsProperties1.Engine = "aurora"
			})

			It("makes the proper calls", func() {
				_, err := Deprovision()
				Expect(dbCluster.DeleteCalled).To(BeTrue())
				Expect(dbCluster.DeleteID).To(Equal(dbClusterIdentifier))
				Expect(dbCluster.DeleteSkipFinalSnapshot).To(BeTrue())
				Expect(dbInstance.DeleteSkipFinalSnapshot).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when it does not skip final snaphot", func() {
				BeforeEach(func() {
					rdsProperties1.SkipFinalSnapshot = false
				})

				It("makes the proper calls", func() {
					_, err := Deprovision()
					Expect(dbCluster.DeleteSkipFinalSnapshot).To(BeFalse())
					Expect(dbInstance.DeleteSkipFinalSnapshot).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when deleting the DB Instance fails", func() {
				BeforeEach(func() {
					dbCluster.DeleteError = errors.New("operation failed")
				})

				It("does not return an error", func() {
					_, err := Deprovision()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})

	var _ = Describe("Bind", func() {
		var (
			bindDetails brokerapi.BindDetails
			dbUsername string
			instance *internaldb.DBInstance
		)

		BeforeEach(func() {
			bindDetails = brokerapi.BindDetails{
				ServiceID:  "Service-1",
				PlanID:     "Plan-1",
				AppGUID:    "Application-1",
				RawParameters: json.RawMessage(""),
			}
			dbUsername = "uApplication_1"

			dbInstance.DescribeDBInstanceDetails = awsrds.DBInstanceDetails{
				Identifier:     dbInstanceIdentifier,
				Address:        "endpoint-address",
				Port:           3306,
			}

			dbCluster.DescribeDBClusterDetails = awsrds.DBClusterDetails{
				Identifier:     dbClusterIdentifier,
				Endpoint:       "endpoint-address",
				Port:           3306,
			}
			instance = MakeInstance()
		})

		Bind := func() (brokerapi.Binding, error) {
			return rdsBroker.Bind(context.Background(), instanceID, bindingID, bindDetails)
		}

		It("returns the proper response", func() {
			bindingResponse, err := Bind()
			Expect(err).ToNot(HaveOccurred())
			credentials := bindingResponse.Credentials.(*CredentialsHash)
			Expect(bindingResponse.SyslogDrainURL).To(BeEmpty())
			Expect(credentials.Host).To(Equal("endpoint-address"))
			Expect(credentials.Port).To(Equal(int64(3306)))
			Expect(credentials.Name).To(Equal(dbName))
			Expect(credentials.Username).To(Equal(dbUsername))
			Expect(credentials.Password).ToNot(BeEmpty())
			Expect(credentials.URI).To(ContainSubstring("@endpoint-address:3306/%s?reconnect=true", dbName))
			Expect(credentials.JDBCURI).To(ContainSubstring("jdbc:fake://endpoint-address:3306/%s?user=%s&password=", dbName, credentials.Username))
		})

		It("makes the proper calls", func() {
			bindingResponse, err := Bind()
			Expect(err).ToNot(HaveOccurred())
			credentials := bindingResponse.Credentials.(*CredentialsHash)
			Expect(dbCluster.DescribeCalled).To(BeFalse())
			Expect(dbInstance.DescribeCalled).To(BeTrue())
			Expect(dbInstance.DescribeID).To(Equal(dbInstanceIdentifier))
			Expect(sqlProvider.GetSQLEngineCalled).To(BeTrue())
			Expect(sqlProvider.GetSQLEngineEngine).To(Equal("test-engine-1"))
			Expect(sqlEngine.OpenCalled).To(BeTrue())
			Expect(sqlEngine.OpenAddress).To(Equal("endpoint-address"))
			Expect(sqlEngine.OpenPort).To(Equal(int64(3306)))
			Expect(sqlEngine.OpenDBName).To(Equal(dbName))
			Expect(sqlEngine.OpenUsername).ToNot(BeEmpty())
			Expect(sqlEngine.OpenPassword).ToNot(BeEmpty())
			Expect(sqlEngine.CreateDBCalled).To(BeFalse())
			Expect(sqlEngine.CreateUserCalled).To(BeTrue())
			Expect(sqlEngine.CreateUserUsername).To(Equal(credentials.Username))
			Expect(sqlEngine.CreateUserPassword).To(Equal(credentials.Password))
			Expect(sqlEngine.GrantPrivilegesCalled).To(BeTrue())
			Expect(sqlEngine.GrantPrivilegesDBName).To(Equal(dbName))
			Expect(sqlEngine.GrantPrivilegesUsername).To(Equal(credentials.Username))
			Expect(sqlEngine.CloseCalled).To(BeTrue())
		})

		Context("when Parameters are not valid", func() {
			BeforeEach(func() {
				bindDetails.RawParameters = json.RawMessage("{\"username\": true}")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("json: cannot unmarshal bool into Go value of type string"))
			})

			Context("and user bind parameters are not allowed", func() {
				BeforeEach(func() {
					allowUserBindParameters = false
				})

				It("does not return an error", func() {
					_, err := Bind()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when Service is not found", func() {
			BeforeEach(func() {
				bindDetails.ServiceID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service 'unknown' not found"))
			})
		})

		Context("when Service is not bindable", func() {
			BeforeEach(func() {
				serviceBindable = false
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service is not bindable"))
			})
		})

		Context("when Service Plan is not found", func() {
			BeforeEach(func() {
				bindDetails.PlanID = "unknown"
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service Plan 'unknown' not found"))
			})
		})

		Context("When given a custom username", func() {
			BeforeEach(func() {
				bindDetails.RawParameters = json.RawMessage("{\"username\": \"custom_user\"}")
			})

			It("returns the proper response", func() {
				bindingResponse, err := Bind()
				Expect(err).ToNot(HaveOccurred())
				credentials := bindingResponse.Credentials.(*CredentialsHash)
				Expect(credentials.Username).To(Equal("custom_user"))
			})

			It("makes the proper calls", func() {
				_, err := Bind()
				Expect(err).ToNot(HaveOccurred())
				Expect(sqlEngine.CreateUserCalled).To(BeTrue())
				Expect(sqlEngine.CreateUserUsername).To(Equal("custom_user"))
				Expect(sqlEngine.GrantPrivilegesCalled).To(BeTrue())
				Expect(sqlEngine.GrantPrivilegesUsername).To(Equal("custom_user"))
			})

			Context("that's invalid", func() {
				BeforeEach(func() {
					bindDetails.RawParameters = json.RawMessage("{\"username\": \"****\"}")
				})

				It("returns the proper error", func() {
					_, err := Bind()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Username must begin with a letter and contain only alphanumeric characters"))
				})
			})

			Context("that already exists", func() {
				var (
					password string
				)
				BeforeEach(func() {
					user, _, err := instance.Bind(internalDB, "binding-zero", "custom_user", internaldb.Standard, encryptionKey)
					Expect(err).NotTo(HaveOccurred())
					password, err = user.Password(encryptionKey)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the proper response", func() {
					bindingResponse, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					credentials := bindingResponse.Credentials.(*CredentialsHash)
					Expect(credentials.Username).To(Equal("custom_user"))
					Expect(credentials.Password).To(Equal(password))
				})

				It("makes the proper calls", func() {
					_, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					Expect(sqlEngine.CreateUserCalled).To(BeFalse())
					Expect(sqlEngine.GrantPrivilegesCalled).To(BeFalse())
				})

				It("makes the proper database entries", func() {
					_, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					instance := internaldb.FindInstance(internalDB, instanceID)
					Expect(instance).NotTo(BeNil())
					// one Master, one Standard
					Expect(instance.Users).To(HaveLen(2))
					Expect(instance.User("custom_user").Bindings).To(HaveLen(2))
				})
			})
		})

		Context("when describing the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.DescribeError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})

			Context("when the DB Instance does not exists", func() {
				BeforeEach(func() {
					dbInstance.DescribeError = awsrds.ErrDBInstanceDoesNotExist
				})

				It("returns the proper error", func() {
					_, err := Bind()
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
				})
			})
		})

		Context("when shared instance", func() {
			BeforeEach(func() {
				rdsProperties1.Shared = true
			})

			Context("with postgres", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "postgres"
					Expect(sharedPostgres.Open("shared-endpoint", 1234, dbName, "master-username", "master-password", config.Verify)).To(Succeed())
				})

				It("returns the proper response", func() {
					bindingResponse, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					credentials := bindingResponse.Credentials.(*CredentialsHash)
					Expect(bindingResponse.SyslogDrainURL).To(BeEmpty())
					Expect(credentials.Host).To(Equal("shared-endpoint"))
					Expect(credentials.Port).To(Equal(int64(1234)))
					Expect(credentials.Name).To(Equal(dbName))
					Expect(credentials.Username).To(Equal(dbUsername))
					Expect(credentials.Password).ToNot(BeEmpty())
					Expect(credentials.Password).ToNot(Equal("master-password"))
					Expect(credentials.URI).To(ContainSubstring("@shared-endpoint:1234/%s?reconnect=true", dbName))
					Expect(credentials.JDBCURI).To(ContainSubstring("jdbc:fake://shared-endpoint:1234/%s?user=%s&password=", dbName, credentials.Username))
				})

				It("makes the proper calls", func() {
					bindingResponse, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					credentials := bindingResponse.Credentials.(*CredentialsHash)
					Expect(dbCluster.DescribeCalled).To(BeFalse())
					Expect(dbInstance.DescribeCalled).To(BeFalse())
					Expect(sqlProvider.GetSQLEngineCalled).To(BeFalse())
					Expect(sharedPostgres.CreateDBCalled).To(BeFalse())
					Expect(sharedPostgres.CreateUserCalled).To(BeTrue())
					Expect(sharedPostgres.CreateUserUsername).To(Equal(credentials.Username))
					Expect(sharedPostgres.CreateUserPassword).To(Equal(credentials.Password))
					Expect(sharedPostgres.GrantPrivilegesCalled).To(BeTrue())
					Expect(sharedPostgres.GrantPrivilegesDBName).To(Equal(dbName))
					Expect(sharedPostgres.GrantPrivilegesUsername).To(Equal(credentials.Username))
					Expect(sharedPostgres.CloseCalled).To(BeFalse())
				})
			})

			Context("with mysql", func() {
				BeforeEach(func() {
					rdsProperties1.Engine = "mysql"
					Expect(sharedMysql.Open("shared-endpoint", 1234, dbName, "master-username", "master-password", config.Verify)).To(Succeed())
				})

				It("returns the proper response", func() {
					bindingResponse, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					credentials := bindingResponse.Credentials.(*CredentialsHash)
					Expect(bindingResponse.SyslogDrainURL).To(BeEmpty())
					Expect(credentials.Host).To(Equal("shared-endpoint"))
					Expect(credentials.Port).To(Equal(int64(1234)))
					Expect(credentials.Name).To(Equal(dbName))
					Expect(credentials.Username).To(Equal(dbUsername))
					Expect(credentials.Password).ToNot(BeEmpty())
					Expect(credentials.Password).ToNot(Equal("master-password"))
					Expect(credentials.URI).To(ContainSubstring("@shared-endpoint:1234/%s?reconnect=true", dbName))
					Expect(credentials.JDBCURI).To(ContainSubstring("jdbc:fake://shared-endpoint:1234/%s?user=%s&password=", dbName, credentials.Username))
				})

				It("makes the proper calls", func() {
					bindingResponse, err := Bind()
					Expect(err).ToNot(HaveOccurred())
					credentials := bindingResponse.Credentials.(*CredentialsHash)
					Expect(dbCluster.DescribeCalled).To(BeFalse())
					Expect(dbInstance.DescribeCalled).To(BeFalse())
					Expect(sqlProvider.GetSQLEngineCalled).To(BeFalse())
					Expect(sharedMysql.CreateDBCalled).To(BeFalse())
					Expect(sharedMysql.CreateUserCalled).To(BeTrue())
					Expect(sharedMysql.CreateUserUsername).To(Equal(credentials.Username))
					Expect(sharedMysql.CreateUserPassword).To(Equal(credentials.Password))
					Expect(sharedMysql.GrantPrivilegesCalled).To(BeTrue())
					Expect(sharedMysql.GrantPrivilegesDBName).To(Equal(dbName))
					Expect(sharedMysql.GrantPrivilegesUsername).To(Equal(credentials.Username))
					Expect(sharedMysql.CloseCalled).To(BeFalse())
				})
			})
		})

		Context("when Engine is aurora", func() {
			BeforeEach(func() {
				rdsProperties1.Engine = "aurora"
			})

			It("does not describe the DB Instance", func() {
				_, err := Bind()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.DescribeCalled).To(BeFalse())
			})

			Context("when describing the DB Cluster fails", func() {
				BeforeEach(func() {
					dbCluster.DescribeError = errors.New("operation failed")
				})

				It("returns the proper error", func() {
					_, err := Bind()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("operation failed"))
				})

				Context("when the DB Cluster does not exists", func() {
					BeforeEach(func() {
						dbCluster.DescribeError = awsrds.ErrDBInstanceDoesNotExist
					})

					It("returns the proper error", func() {
						_, err := Bind()
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
					})
				})
			})
		})

		Context("when getting the SQL Engine fails", func() {
			BeforeEach(func() {
				sqlProvider.GetSQLEngineError = errors.New("Engine 'unknown' not supported")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Engine 'unknown' not supported"))
			})
		})

		Context("when opening a DB connection fails", func() {
			BeforeEach(func() {
				sqlEngine.OpenError = errors.New("Failed to open sqlEngine")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to open sqlEngine"))
			})
		})

		Context("when creating a DB user fails", func() {
			BeforeEach(func() {
				sqlEngine.CreateUserError = errors.New("Failed to create user")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to create user"))
				Expect(sqlEngine.CloseCalled).To(BeTrue())
			})
		})

		Context("when granting privileges fails", func() {
			BeforeEach(func() {
				sqlEngine.GrantPrivilegesError = errors.New("Failed to grant privileges")
			})

			It("returns the proper error", func() {
				_, err := Bind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to grant privileges"))
				Expect(sqlEngine.CloseCalled).To(BeTrue())
			})
		})
	})

	var _ = Describe("Unbind", func() {
		var (
			unbindDetails brokerapi.UnbindDetails
			dbUsername string
		)

		BeforeEach(func() {
			unbindDetails = brokerapi.UnbindDetails{
				ServiceID: "Service-1",
				PlanID:    "Plan-1",
			}

			dbInstance.DescribeDBInstanceDetails = awsrds.DBInstanceDetails{
				Identifier:     dbInstanceIdentifier,
				Address:        "endpoint-address",
				Port:           3306,
			}

			dbCluster.DescribeDBClusterDetails = awsrds.DBClusterDetails{
				Identifier:     dbClusterIdentifier,
				Endpoint:       "endpoint-address",
				Port:           3306,
			}
			instance := MakeInstance()
			dbUsername = "username"
			_, _, err := instance.Bind(internalDB, bindingID, dbUsername, internaldb.Standard, encryptionKey)
			Expect(err).NotTo(HaveOccurred())
		})

		Unbind := func() (error) {
			return rdsBroker.Unbind(context.Background(), instanceID, bindingID, unbindDetails)
		}

		It("makes the proper calls", func() {
			err := Unbind()
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCluster.DescribeCalled).To(BeFalse())
			Expect(dbInstance.DescribeCalled).To(BeTrue())
			Expect(dbInstance.DescribeID).To(Equal(dbInstanceIdentifier))
			Expect(sqlProvider.GetSQLEngineCalled).To(BeTrue())
			Expect(sqlProvider.GetSQLEngineEngine).To(Equal("test-engine-1"))
			Expect(sqlEngine.OpenCalled).To(BeTrue())
			Expect(sqlEngine.OpenAddress).To(Equal("endpoint-address"))
			Expect(sqlEngine.OpenPort).To(Equal(int64(3306)))
			Expect(sqlEngine.OpenDBName).To(Equal(dbName))
			Expect(sqlEngine.OpenUsername).ToNot(BeEmpty())
			Expect(sqlEngine.OpenPassword).ToNot(BeEmpty())
			Expect(sqlEngine.RevokePrivilegesCalled).To(BeTrue())
			Expect(sqlEngine.RevokePrivilegesUsername).To(Equal(dbUsername))
			Expect(sqlEngine.RevokePrivilegesDBName).To(Equal(dbName))
			Expect(sqlEngine.DropDBCalled).To(BeFalse())
			Expect(sqlEngine.DropUserCalled).To(BeTrue())
			Expect(sqlEngine.DropUserUsername).To(Equal(dbUsername))
			Expect(sqlEngine.CloseCalled).To(BeTrue())
		})

		Context("when another binding has the same username", func() {
			BeforeEach(func() {
				instance := internaldb.FindInstance(internalDB, instanceID)
				_, _, err := instance.Bind(internalDB, "binding-two", dbUsername, internaldb.Standard, encryptionKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("makes the proper calls", func() {
				err := Unbind()
				Expect(err).ToNot(HaveOccurred())
				Expect(sqlEngine.RevokePrivilegesCalled).To(BeFalse())
				Expect(sqlEngine.DropUserCalled).To(BeFalse())
			})
		})

		Context("when Service Plan is not found", func() {
			BeforeEach(func() {
				unbindDetails.PlanID = "unknown"
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Service Plan 'unknown' not found"))
			})
		})

		Context("when describing the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.DescribeError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})

			Context("when the DB Instance does not exists", func() {
				BeforeEach(func() {
					dbInstance.DescribeError = awsrds.ErrDBInstanceDoesNotExist
				})

				It("returns the proper error", func() {
					err := Unbind()
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
				})
			})
		})

		Context("when shared instance", func() {
			BeforeEach(func() {
				BeforeEach(func() {
					rdsProperties1.Shared = true
				})

				Context("with postgres", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "postgres"
					})

					It("makes the proper calls", func() {
						err := Unbind()
						Expect(err).ToNot(HaveOccurred())
						Expect(dbCluster.DescribeCalled).To(BeFalse())
						Expect(dbInstance.DescribeCalled).To(BeFalse())
						Expect(sqlProvider.GetSQLEngineCalled).To(BeFalse())
						Expect(sharedPostgres.OpenCalled).To(BeFalse())
						Expect(sharedPostgres.RevokePrivilegesCalled).To(BeTrue())
						Expect(sharedPostgres.RevokePrivilegesUsername).To(Equal(dbUsername))
						Expect(sharedPostgres.RevokePrivilegesDBName).To(Equal(dbName))
						Expect(sharedPostgres.DropDBCalled).To(BeFalse())
						Expect(sharedPostgres.DropUserCalled).To(BeTrue())
						Expect(sharedPostgres.DropUserUsername).To(Equal(dbUsername))
						Expect(sharedPostgres.CloseCalled).To(BeFalse())
					})
				})

				Context("with mysql", func() {
					BeforeEach(func() {
						rdsProperties1.Engine = "mysql"
					})

					It("makes the proper calls", func() {
						err := Unbind()
						Expect(err).ToNot(HaveOccurred())
						Expect(dbCluster.DescribeCalled).To(BeFalse())
						Expect(dbInstance.DescribeCalled).To(BeFalse())
						Expect(sqlProvider.GetSQLEngineCalled).To(BeFalse())
						Expect(sharedMysql.OpenCalled).To(BeFalse())
						Expect(sharedMysql.RevokePrivilegesCalled).To(BeTrue())
						Expect(sharedMysql.RevokePrivilegesUsername).To(Equal(dbUsername))
						Expect(sharedMysql.RevokePrivilegesDBName).To(Equal(dbName))
						Expect(sharedMysql.DropDBCalled).To(BeFalse())
						Expect(sharedMysql.DropUserCalled).To(BeTrue())
						Expect(sharedMysql.DropUserUsername).To(Equal(dbUsername))
						Expect(sharedMysql.CloseCalled).To(BeFalse())
					})
				})
			})
		})

		Context("when Engine is aurora", func() {
			BeforeEach(func() {
				rdsProperties1.Engine = "aurora"
			})

			It("does not describe the DB Instance", func() {
				err := Unbind()
				Expect(err).ToNot(HaveOccurred())
				Expect(dbInstance.DescribeCalled).To(BeFalse())
			})

			Context("when describing the DB Cluster fails", func() {
				BeforeEach(func() {
					dbCluster.DescribeError = errors.New("operation failed")
				})

				It("returns the proper error", func() {
					err := Unbind()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("operation failed"))
				})

				Context("when the DB Cluster does not exists", func() {
					BeforeEach(func() {
						dbCluster.DescribeError = awsrds.ErrDBInstanceDoesNotExist
					})

					It("returns the proper error", func() {
						err := Unbind()
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
					})
				})
			})
		})

		Context("when getting the SQL Engine fails", func() {
			BeforeEach(func() {
				sqlProvider.GetSQLEngineError = errors.New("SQL Engine 'unknown' not supported")
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("SQL Engine 'unknown' not supported"))
			})
		})

		Context("when opening a DB connection fails", func() {
			BeforeEach(func() {
				sqlEngine.OpenError = errors.New("Failed to open sqlEngine")
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to open sqlEngine"))
			})
		})

		Context("when revoking privileges fails", func() {
			BeforeEach(func() {
				sqlEngine.RevokePrivilegesError = errors.New("Failed to revoke privileges")
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to revoke privileges"))
				Expect(sqlEngine.CloseCalled).To(BeTrue())
			})
		})

		Context("when deleting a user fails", func() {
			BeforeEach(func() {
				sqlEngine.DropUserError = errors.New("Failed to delete user")
			})

			It("returns the proper error", func() {
				err := Unbind()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Failed to delete user"))
				Expect(sqlEngine.CloseCalled).To(BeTrue())
			})
		})
	})

	var _ = Describe("LastOperation", func() {
		var (
			dbInstanceStatus            string
			lastOperationState          brokerapi.LastOperationState
			properLastOperationResponse brokerapi.LastOperation
		)

		JustBeforeEach(func() {
			dbInstance.DescribeDBInstanceDetails = awsrds.DBInstanceDetails{
				Identifier:     dbInstanceIdentifier,
				Engine:         "test-engine",
				Address:        "endpoint-address",
				Port:           3306,
				DBName:         "test-db",
				Status:         dbInstanceStatus,
			}

			properLastOperationResponse = brokerapi.LastOperation{
				State:       lastOperationState,
				Description: "DB Instance '" + dbInstanceIdentifier + "' status is '" + dbInstanceStatus + "'",
			}
			MakeInstance()
		})

		LastOperation := func() (brokerapi.LastOperation, error) {
			return rdsBroker.LastOperation(context.Background(), instanceID, "")
		}

		Context("when describing the DB Instance fails", func() {
			BeforeEach(func() {
				dbInstance.DescribeError = errors.New("operation failed")
			})

			It("returns the proper error", func() {
				_, err := LastOperation()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("operation failed"))
			})

			It("does not delete the local instance", func() {
				_, err := LastOperation()
				Expect(err).To(HaveOccurred())
				Expect(internaldb.FindInstance(internalDB, instanceID)).NotTo(BeNil())
			})

			Context("when the DB Instance does not exists", func() {
				BeforeEach(func() {
					dbInstance.DescribeError = awsrds.ErrDBInstanceDoesNotExist
				})

				It("returns the proper error", func() {
					_, err := LastOperation()
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(brokerapi.ErrInstanceDoesNotExist))
				})

				It("deletes the local instance", func() {
					_, err := LastOperation()
					Expect(err).To(HaveOccurred())
					Expect(internaldb.FindInstance(internalDB, instanceID)).To(BeNil())
				})
			})
		})

		Context("when last operation is still in progress", func() {
			BeforeEach(func() {
				dbInstanceStatus = "creating"
				lastOperationState = brokerapi.InProgress
			})

			It("returns the proper LastOperationResponse", func() {
				lastOperationResponse, err := LastOperation()
				Expect(err).ToNot(HaveOccurred())
				Expect(lastOperationResponse).To(Equal(properLastOperationResponse))
			})
		})

		Context("when last operation failed", func() {
			BeforeEach(func() {
				dbInstanceStatus = "failed"
				lastOperationState = brokerapi.Failed
			})

			It("returns the proper LastOperationResponse", func() {
				lastOperationResponse, err := LastOperation()
				Expect(err).ToNot(HaveOccurred())
				Expect(lastOperationResponse).To(Equal(properLastOperationResponse))
			})
		})

		Context("when last operation succeeded", func() {
			BeforeEach(func() {
				dbInstanceStatus = "available"
				lastOperationState = brokerapi.Succeeded
			})

			It("returns the proper LastOperationResponse", func() {
				lastOperationResponse, err := LastOperation()
				Expect(err).ToNot(HaveOccurred())
				Expect(lastOperationResponse).To(Equal(properLastOperationResponse))
			})

			Context("but has pending modifications", func() {
				JustBeforeEach(func() {
					dbInstance.DescribeDBInstanceDetails.PendingModifications = true

					properLastOperationResponse = brokerapi.LastOperation{
						State:       brokerapi.InProgress,
						Description: "DB Instance '" + dbInstanceIdentifier + "' has pending modifications",
					}
				})

				It("returns the proper LastOperationResponse", func() {
					lastOperationResponse, err := LastOperation()
					Expect(err).ToNot(HaveOccurred())
					Expect(lastOperationResponse).To(Equal(properLastOperationResponse))
				})
			})
		})

		Context("when shared instance", func() {
			BeforeEach(func() {
				rdsProperties1.Shared = true
			})

			It("returns failed", func() {
				lastOperationResponse, err := LastOperation()
				Expect(err).ToNot(HaveOccurred())
				Expect(lastOperationResponse.State).To(Equal(brokerapi.Failed))
			})
		})
	})
})

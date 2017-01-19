package rdsbroker

import (
	"fmt"
	"strings"
)

/* As much as it would be nice to use brokerapi.Service here rather than redefining
 * the whole structure ourselves, the version in brokerapi is specifically
 * replicating the structure sent to cloud foundry. And we want to add a few more
 * options for defining the RDS properties. And brokerapi doesn't let us dump it in
 * metadata.
 *
 * TODO: Find a better way
 */
type Catalog struct {
	Services []Service `yaml:"services,omitempty"`
}

type Service struct {
	ID              string           `yaml:"id"`
	Name            string           `yaml:"name"`
	Description     string           `yaml:"description"`
	Bindable        bool             `yaml:"bindable,omitempty"`
	Tags            []string         `yaml:"tags,omitempty"`
	Metadata        *ServiceMetadata `yaml:"metadata,omitempty"`
	Requires        []string         `yaml:"requires,omitempty"`
	PlanUpdateable  bool             `yaml:"plan_updateable"`
	Plans           []ServicePlan    `yaml:"plans,omitempty"`
	DashboardClient *DashboardClient `yaml:"dashboard_client,omitempty"`
}

type ServiceMetadata struct {
	DisplayName         string `yaml:"displayName,omitempty"`
	ImageURL            string `yaml:"imageUrl,omitempty"`
	LongDescription     string `yaml:"longDescription,omitempty"`
	ProviderDisplayName string `yaml:"providerDisplayName,omitempty"`
	DocumentationURL    string `yaml:"documentationUrl,omitempty"`
	SupportURL          string `yaml:"supportUrl,omitempty"`
}

type ServicePlan struct {
	ID            string               `yaml:"id"`
	Name          string               `yaml:"name"`
	Description   string               `yaml:"description"`
	Metadata      *ServicePlanMetadata `yaml:"metadata,omitempty"`
	Free          bool                 `yaml:"free"`
	RDSProperties RDSProperties        `yaml:"rds_properties,omitempty"`
}

type ServicePlanMetadata struct {
	Bullets     []string `yaml:"bullets,omitempty"`
	Costs       []Cost   `yaml:"costs,omitempty"`
	DisplayName string   `yaml:"displayName,omitempty"`
}

type DashboardClient struct {
	ID          string `yaml:"id,omitempty"`
	Secret      string `yaml:"secret,omitempty"`
	RedirectURI string `yaml:"redirect_uri,omitempty"`
}

type Cost struct {
	Amount map[string]interface{} `yaml:"amount,omitempty"`
	Unit   string                 `yaml:"unit,omitempty"`
}

type RDSProperties struct {
	DBInstanceClass             string   `yaml:"db_instance_class"`
	Engine                      string   `yaml:"engine"`
	EngineVersion               string   `yaml:"engine_version"`
	AllocatedStorage            int64    `yaml:"allocated_storage"`
	AutoMinorVersionUpgrade     bool     `yaml:"auto_minor_version_upgrade,omitempty"`
	AvailabilityZone            string   `yaml:"availability_zone,omitempty"`
	BackupRetentionPeriod       int64    `yaml:"backup_retention_period,omitempty"`
	CharacterSetName            string   `yaml:"character_set_name,omitempty"`
	DBParameterGroupName        string   `yaml:"db_parameter_group_name,omitempty"`
	DBClusterParameterGroupName string   `yaml:"db_cluster_parameter_group_name,omitempty"`
	DBSecurityGroups            []string `yaml:"db_security_groups,omitempty"`
	DBSubnetGroupName           string   `yaml:"db_subnet_group_name,omitempty"`
	LicenseModel                string   `yaml:"license_model,omitempty"`
	MultiAZ                     bool     `yaml:"multi_az,omitempty"`
	OptionGroupName             string   `yaml:"option_group_name,omitempty"`
	Port                        int64    `yaml:"port,omitempty"`
	PreferredBackupWindow       string   `yaml:"preferred_backup_window,omitempty"`
	PreferredMaintenanceWindow  string   `yaml:"preferred_maintenance_window,omitempty"`
	PubliclyAccessible          bool     `yaml:"publicly_accessible,omitempty"`
	StorageEncrypted            bool     `yaml:"storage_encrypted,omitempty"`
	KmsKeyID                    string   `yaml:"kms_key_id,omitempty"`
	StorageType                 string   `yaml:"storage_type,omitempty"`
	Iops                        int64    `yaml:"iops,omitempty"`
	VpcSecurityGroupIds         []string `yaml:"vpc_security_group_ids,omitempty"`
	CopyTagsToSnapshot          bool     `yaml:"copy_tags_to_snapshot,omitempty"`
	SkipFinalSnapshot           bool     `yaml:"skip_final_snapshot,omitempty"`
}

func (c Catalog) Validate() error {
	for _, service := range c.Services {
		if err := service.Validate(); err != nil {
			return fmt.Errorf("Validating Services configuration: %s", err)
		}
	}

	return nil
}

func (c Catalog) FindService(serviceID string) (service Service, found bool) {
	for _, service := range c.Services {
		if service.ID == serviceID {
			return service, true
		}
	}

	return service, false
}

func (c Catalog) FindServicePlan(planID string) (plan ServicePlan, found bool) {
	for _, service := range c.Services {
		for _, plan := range service.Plans {
			if plan.ID == planID {
				return plan, true
			}
		}
	}

	return plan, false
}

func (s Service) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("Must provide a non-empty ID (%+v)", s)
	}

	if s.Name == "" {
		return fmt.Errorf("Must provide a non-empty Name (%+v)", s)
	}

	if s.Description == "" {
		return fmt.Errorf("Must provide a non-empty Description (%+v)", s)
	}

	for _, servicePlan := range s.Plans {
		if err := servicePlan.Validate(); err != nil {
			return fmt.Errorf("Validating Plans configuration: %s", err)
		}
	}

	return nil
}

func (sp ServicePlan) Validate() error {
	if sp.ID == "" {
		return fmt.Errorf("Must provide a non-empty ID (%+v)", sp)
	}

	if sp.Name == "" {
		return fmt.Errorf("Must provide a non-empty Name (%+v)", sp)
	}

	if sp.Description == "" {
		return fmt.Errorf("Must provide a non-empty Description (%+v)", sp)
	}

	if err := sp.RDSProperties.Validate(); err != nil {
		return fmt.Errorf("Validating RDS Properties configuration: %s", err)
	}

	return nil
}

func (rp RDSProperties) Validate() error {
	if rp.DBInstanceClass == "" {
		return fmt.Errorf("Must provide a non-empty DBInstanceClass (%+v)", rp)
	}

	if rp.Engine == "" {
		return fmt.Errorf("Must provide a non-empty Engine (%+v)", rp)
	}

	switch strings.ToLower(rp.Engine) {
	case "aurora":
	case "mariadb":
	case "mysql":
	case "postgres":
	default:
		return fmt.Errorf("This broker does not support RDS engine '%s' (%+v)", rp.Engine, rp)
	}

	return nil
}

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
	ID              string           `json:"id" yaml:"id"`
	Name            string           `json:"name" yaml:"name"`
	Description     string           `json:"description" yaml:"description"`
	Bindable        bool             `json:"bindable,omitempty" yaml:"bindable,omitempty"`
	Tags            []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
	Metadata        *ServiceMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Requires        []string         `json:"requires,omitempty" yaml:"requires,omitempty"`
	PlanUpdateable  bool             `json:"plan_updateable" yaml:"plan_updateable"`
	Plans           []ServicePlan    `json:"plans,omitempty" yaml:"plans,omitempty"`
	DashboardClient *DashboardClient `json:"dashboard_client,omitempty" yaml:"dashboard_client,omitempty"`
}

type ServiceMetadata struct {
	DisplayName         string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ImageURL            string `json:"imageUrl,omitempty" yaml:"imageUrl,omitempty"`
	LongDescription     string `json:"longDescription,omitempty" yaml:"longDescription,omitempty"`
	ProviderDisplayName string `json:"providerDisplayName,omitempty" yaml:"providerDisplayName,omitempty"`
	DocumentationURL    string `json:"documentationUrl,omitempty" yaml:"documentationUrl,omitempty"`
	SupportURL          string `json:"supportUrl,omitempty" yaml:"supportUrl,omitempty"`
}

type ServicePlan struct {
	ID            string               `json:"id" yaml:"id"`
	Name          string               `json:"name" yaml:"name"`
	Description   string               `json:"description" yaml:"description"`
	Metadata      *ServicePlanMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Free          *bool                `json:"free" yaml:"free"`
	RDSProperties RDSProperties        `json:"rds_properties,omitempty" yaml:"rds_properties,omitempty"`
}

type ServicePlanMetadata struct {
	Bullets     []string `json:"bullets,omitempty" yaml:"bullets,omitempty"`
	Costs       []Cost   `json:"costs,omitempty" yaml:"costs,omitempty"`
	DisplayName string   `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}

type DashboardClient struct {
	ID          string `json:"id,omitempty" yaml:"id,omitempty"`
	Secret      string `json:"secret,omitempty" yaml:"secret,omitempty"`
	RedirectURI string `json:"redirect_uri,omitempty" yaml:"redirect_uri,omitempty"`
}

type Cost struct {
	Amount map[string]interface{} `json:"amount,omitempty" yaml:"amount,omitempty"`
	Unit   string                 `json:"unit,omitempty" yaml:"unit,omitempty"`
}

type RDSProperties struct {
	DBInstanceClass             string   `json:"db_instance_class" yaml:"db_instance_class"`
	Engine                      string   `json:"engine" yaml:"engine"`
	EngineVersion               string   `json:"engine_version" yaml:"engine_version"`
	AllocatedStorage            int64    `json:"allocated_storage" yaml:"allocated_storage"`
	AutoMinorVersionUpgrade     bool     `json:"auto_minor_version_upgrade,omitempty" yaml:"auto_minor_version_upgrade,omitempty"`
	AvailabilityZone            string   `json:"availability_zone,omitempty" yaml:"availability_zone,omitempty"`
	BackupRetentionPeriod       int64    `json:"backup_retention_period,omitempty" yaml:"backup_retention_period,omitempty"`
	CharacterSetName            string   `json:"character_set_name,omitempty" yaml:"character_set_name,omitempty"`
	DBParameterGroupName        string   `json:"db_parameter_group_name,omitempty" yaml:"db_parameter_group_name,omitempty"`
	DBClusterParameterGroupName string   `json:"db_cluster_parameter_group_name,omitempty" yaml:"db_cluster_parameter_group_name,omitempty"`
	DBSecurityGroups            []string `json:"db_security_groups,omitempty" yaml:"db_security_groups,omitempty"`
	DBSubnetGroupName           string   `json:"db_subnet_group_name,omitempty" yaml:"db_subnet_group_name,omitempty"`
	LicenseModel                string   `json:"license_model,omitempty" yaml:"license_model,omitempty"`
	MultiAZ                     bool     `json:"multi_az,omitempty" yaml:"multi_az,omitempty"`
	OptionGroupName             string   `json:"option_group_name,omitempty" yaml:"option_group_name,omitempty"`
	Port                        int64    `json:"port,omitempty" yaml:"port,omitempty"`
	PreferredBackupWindow       string   `json:"preferred_backup_window,omitempty" yaml:"preferred_backup_window,omitempty"`
	PreferredMaintenanceWindow  string   `json:"preferred_maintenance_window,omitempty" yaml:"preferred_maintenance_window,omitempty"`
	PubliclyAccessible          bool     `json:"publicly_accessible,omitempty" yaml:"publicly_accessible,omitempty"`
	StorageEncrypted            bool     `json:"storage_encrypted,omitempty" yaml:"storage_encrypted,omitempty"`
	KmsKeyID                    string   `json:"kms_key_id,omitempty" yaml:"kms_key_id,omitempty"`
	StorageType                 string   `json:"storage_type,omitempty" yaml:"storage_type,omitempty"`
	Iops                        int64    `json:"iops,omitempty" yaml:"iops,omitempty"`
	VpcSecurityGroupIds         []string `json:"vpc_security_group_ids,omitempty" yaml:"vpc_security_group_ids,omitempty"`
	CopyTagsToSnapshot          bool     `json:"copy_tags_to_snapshot,omitempty" yaml:"copy_tags_to_snapshot,omitempty"`
	SkipFinalSnapshot           bool     `json:"skip_final_snapshot,omitempty" yaml:"skip_final_snapshot,omitempty"`
	Shared                      bool     `json:"shared" yaml:"shared"`
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

func (c Catalog) FindServicePlan(serviceID, planID string) (plan ServicePlan, found bool) {
	service, found := c.FindService(serviceID)
	if !found {
		return plan, false
	}
	for _, plan := range service.Plans {
		if plan.ID == planID {
			return plan, true
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
		if s.PlanUpdateable && servicePlan.RDSProperties.Shared {
			return fmt.Errorf("Cannot have an updateable service with shared plans (%+v)", s)
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
	if !rp.Shared && rp.DBInstanceClass == "" {
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

	if rp.Shared {
		switch strings.ToLower(rp.Engine) {
		case "mysql":
		case "postgres":
		default:
			return fmt.Errorf("This broker does not support RDS engine '%s' with a shared instance (%+v)", rp.Engine, rp)
		}
	}

	return nil
}

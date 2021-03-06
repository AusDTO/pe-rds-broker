package rdsbroker

/* Currently the provision parameters are a json.RawMessage in brokerapi
 * while the update and bind parameters are a map[string]interface{}
 * There is some interest in changing everything to json.RawMessage
 * https://github.com/pivotal-cf/brokerapi/issues/36
 */
type ProvisionParameters struct {
	BackupRetentionPeriod      int64  `json:"backup_retention_period"`
	CharacterSetName           string `json:"character_set_name"`
	PreferredBackupWindow      string `json:"preferred_backup_window"`
	PreferredMaintenanceWindow string `json:"preferred_maintenance_window"`
}

type UpdateParameters struct {
	ApplyImmediately           bool      `json:"apply_immediately"`
	BackupRetentionPeriod      int64     `json:"backup_retention_period"`
	PreferredBackupWindow      string    `json:"preferred_backup_window"`
	PreferredMaintenanceWindow string    `json:"preferred_maintenance_window"`
	Extensions                 *[]string `json:"extensions"`
}

type BindParameters struct {
	Username string `json:"username"`
}

type CredentialsHash struct {
	Host     string `json:"host,omitempty"`
	Port     int64  `json:"port,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	URI      string `json:"uri,omitempty"`
	JDBCURI  string `json:"jdbcUrl,omitempty"`

	// Some apps expect these alternate names, I'm looking at you Stratos: https://github.com/cloudfoundry-incubator/stratos/blob/v2-master/deploy/cloud-foundry/db-migration/README.md#note-on-service-bindings
	Hostname string `json:"hostname,omitempty"`
	DBName   string `json:"dbname,omitempty"`
}

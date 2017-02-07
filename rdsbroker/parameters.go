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
	ApplyImmediately           bool   `mapstructure:"apply_immediately"`
	BackupRetentionPeriod      int64  `mapstructure:"backup_retention_period"`
	PreferredBackupWindow      string `mapstructure:"preferred_backup_window"`
	PreferredMaintenanceWindow string `mapstructure:"preferred_maintenance_window"`
}

type BindParameters struct {
	Username string `mapstructure:"username"`
}

type CredentialsHash struct {
	Host     string `json:"host,omitempty"`
	Port     int64  `json:"port,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	URI      string `json:"uri,omitempty"`
	JDBCURI  string `json:"jdbcUrl,omitempty"`
}

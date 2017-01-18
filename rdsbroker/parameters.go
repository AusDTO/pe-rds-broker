package rdsbroker

type ProvisionParameters struct {
	BackupRetentionPeriod      int64  `mapstructure:"backup_retention_period"`
	CharacterSetName           string `mapstructure:"character_set_name"`
	DBName                     string `mapstructure:"dbname"`
	PreferredBackupWindow      string `mapstructure:"preferred_backup_window"`
	PreferredMaintenanceWindow string `mapstructure:"preferred_maintenance_window"`
}

type UpdateParameters struct {
	ApplyImmediately           bool   `mapstructure:"apply_immediately"`
	BackupRetentionPeriod      int64  `mapstructure:"backup_retention_period"`
	PreferredBackupWindow      string `mapstructure:"preferred_backup_window"`
	PreferredMaintenanceWindow string `mapstructure:"preferred_maintenance_window"`
}

type BindParameters struct {
	DBName string `mapstructure:"dbname"`
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

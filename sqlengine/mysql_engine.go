package sqlengine

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL Driver

	"code.cloudfoundry.org/lager"
	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/utils"
)

type MySQLEngine struct {
	logger lager.Logger
	db     *sql.DB
	config config.DBConfig
}

func NewMySQLEngine(logger lager.Logger) *MySQLEngine {
	return &MySQLEngine{
		logger: logger.Session("mysql-engine"),
	}
}

func (d *MySQLEngine) Open(conf config.DBConfig) error {
	d.config = conf
	connectionString := d.connectionString()
	d.logger.Debug("sql-open", lager.Data{"connection-string": connectionString})

	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		return err
	}

	d.db = db

	return nil
}

func (d *MySQLEngine) Close() {
	if d.db != nil {
		d.db.Close()
	}
}

func (d *MySQLEngine) ExistsDB(dbname string) (bool, error) {
	selectDatabaseStatement := "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = '" + dbname + "'"
	d.logger.Debug("database-exists", lager.Data{"statement": selectDatabaseStatement})

	var dummy string
	err := d.db.QueryRow(selectDatabaseStatement).Scan(&dummy)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	}

	return true, nil
}

func (d *MySQLEngine) CreateDB(dbname string) error {
	ok, err := d.ExistsDB(dbname)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	createDBStatement := "CREATE DATABASE IF NOT EXISTS " + dbname
	d.logger.Debug("create-database", lager.Data{"statement": createDBStatement})

	if _, err := d.db.Exec(createDBStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) DropDB(dbname string) error {
	dropDBStatement := "DROP DATABASE IF EXISTS " + dbname
	d.logger.Debug("drop-database", lager.Data{"statement": dropDBStatement})

	if _, err := d.db.Exec(dropDBStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) CreateUser(username string, password string) error {
	createUserStatement := "CREATE USER '" + username + "' IDENTIFIED BY '" + password + "'"
	d.logger.Debug("create-user", lager.Data{"statement": createUserStatement})

	if _, err := d.db.Exec(createUserStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) DropUser(username string) error {
	dropUserStatement := "DROP USER '" + username + "'@'%'"
	d.logger.Debug("drop-user", lager.Data{"statement": dropUserStatement})

	if _, err := d.db.Exec(dropUserStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) GrantPrivileges(dbname string, username string) error {
	grantPrivilegesStatement := "GRANT ALL PRIVILEGES ON " + dbname + ".* TO '" + username + "'@'%'"
	d.logger.Debug("grant-privileges", lager.Data{"statement": grantPrivilegesStatement})

	if _, err := d.db.Exec(grantPrivilegesStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) RevokePrivileges(dbname string, username string) error {
	revokePrivilegesStatement := "REVOKE ALL PRIVILEGES ON " + dbname + ".* from '" + username + "'@'%'"
	d.logger.Debug("revoke-privileges", lager.Data{"statement": revokePrivilegesStatement})

	if _, err := d.db.Exec(revokePrivilegesStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *MySQLEngine) SetExtensions(extensions []string) error {
	// mysql doesn't have extensions
	return nil
}

func (d *MySQLEngine) URI(dbname string, username string, password string) string {
	return fmt.Sprintf("mysql://%s:%s@%s:%d/%s?reconnect=true", username, password, d.config.Url, d.config.Port, dbname)
}

func (d *MySQLEngine) JDBCURI(dbname string, username string, password string) string {
	return fmt.Sprintf("jdbc:mysql://%s:%d/%s?user=%s&password=%s", d.config.Url, d.config.Port, dbname, username, password)
}

func (d *MySQLEngine) connectionString() string {
	var tls string
	switch d.config.Sslmode {
	case config.Disable:
		tls = "false"
	case config.RequireNoVerify:
		tls = "skip-verify"
	case config.Verify:
		tls = "true"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s", d.config.Username, d.config.Password, d.config.Url, d.config.Port, d.config.DBName, tls)
}

func (d *MySQLEngine) Config() config.DBConfig {
	return d.config
}

func (d *MySQLEngine) CreateUsername(instanceid string) (string, error) {
	return utils.RandUsername()
}

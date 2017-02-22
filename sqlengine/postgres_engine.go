package sqlengine

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL Driver

	"code.cloudfoundry.org/lager"
	"github.com/AusDTO/pe-rds-broker/config"
	"github.com/AusDTO/pe-rds-broker/utils"
)

type PostgresEngine struct {
	logger  lager.Logger
	db      *sql.DB
	config  config.DBConfig
}

func NewPostgresEngine(logger lager.Logger) *PostgresEngine {
	return &PostgresEngine{
		logger: logger.Session("postgres-engine"),
	}
}

func (d *PostgresEngine) Open(conf config.DBConfig) error {
	d.config = conf
	connectionString := d.connectionString()
	d.logger.Debug("sql-open", lager.Data{"connection-string": connectionString})

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return err
	}

	d.db = db

	return nil
}

func (d *PostgresEngine) Close() {
	if d.db != nil {
		d.db.Close()
	}
}

func (d *PostgresEngine) ExistsDB(dbname string) (bool, error) {
	selectDatabaseStatement := "SELECT datname FROM pg_database WHERE datname='" + dbname + "'"
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

func (d *PostgresEngine) CreateDB(dbname string) error {
	ok, err := d.ExistsDB(dbname)
	if err != nil {
		return err
	}
	if ok {
		d.logger.Debug("db-already-exists", lager.Data{"dbname": dbname})
		return nil
	}

	createDBStatement := "CREATE DATABASE \"" + dbname + "\""
	d.logger.Debug("create-database", lager.Data{"statement": createDBStatement})

	if _, err := d.db.Exec(createDBStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) DropDB(dbname string) error {
	if err := d.dropConnections(dbname); err != nil {
		return err
	}

	dropDBStatement := "DROP DATABASE IF EXISTS \"" + dbname + "\""
	d.logger.Debug("drop-database", lager.Data{"statement": dropDBStatement})

	if _, err := d.db.Exec(dropDBStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) CreateUser(username string, password string) error {
	// If the user has been created and "dropped" previously, the user will
	// still exist but with NOLOGIN
	var exists bool
	err := d.db.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname=$1)", username).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		loginStatement := "ALTER ROLE \"" + username + "\" WITH LOGIN PASSWORD '" + password + "'"
		d.logger.Debug("login", lager.Data{"statement": loginStatement})

		if _, err := d.db.Exec(loginStatement); err != nil {
			d.logger.Error("sql-error", err)
			return err
		}
	} else {
		createUserStatement := "CREATE USER \"" + username + "\" WITH PASSWORD '" + password + "'"
		d.logger.Debug("create-user", lager.Data{"statement": createUserStatement})

		if _, err := d.db.Exec(createUserStatement); err != nil {
			d.logger.Error("sql-error", err)
			return err
		}
	}

	return nil
}

func (d *PostgresEngine) DropUser(username string) error {
	// For PostgreSQL we don't drop the user because it might still be owner of some objects
	// We make it so they can't log in instead

	nologinStatement := "ALTER ROLE \"" + username + "\" WITH NOLOGIN"
	d.logger.Debug("nologin", lager.Data{"statement": nologinStatement})

	if _, err := d.db.Exec(nologinStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) GrantPrivileges(dbname string, username string) error {
	grantPrivilegesStatement := "GRANT ALL PRIVILEGES ON DATABASE \"" + dbname + "\" TO \"" + username + "\""
	d.logger.Debug("grant-privileges", lager.Data{"statement": grantPrivilegesStatement})

	if _, err := d.db.Exec(grantPrivilegesStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) RevokePrivileges(dbname string, username string) error {
	revokePrivilegesStatement := "REVOKE ALL PRIVILEGES ON DATABASE \"" + dbname + "\" FROM \"" + username + "\""
	d.logger.Debug("revoke-privileges", lager.Data{"statement": revokePrivilegesStatement})

	if _, err := d.db.Exec(revokePrivilegesStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) SetExtensions(extensions []string) error {
	// validate extensions
	for _, extension := range extensions {
		if !utils.IsValidExtensionName(extension) {
			return fmt.Errorf("Invalid extension name '%s'", extension)
		}
	}

	// get current extensions
	rows, err := d.db.Query("SELECT extname FROM pg_extension WHERE extname != 'plpgsql'")
	if err != nil {
		return err
	}
	defer rows.Close()
	var oldExtensions []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		oldExtensions = append(oldExtensions, name)
	}
	// Add and remove extensions as required
	// Note: There are more efficient ways to do this involving sorting both lists. But I'm assuming that the lists
	// won't be particularly long in which case this should be fine.
	var found bool
	for _, old := range oldExtensions {
		found = false
		for _, new := range extensions {
			if old == new {
				found = true
				break
			}
		}
		if !found {
			d.logger.Debug("drop-extension", lager.Data{"extension": old})
			if _, err := d.db.Exec(fmt.Sprintf("DROP EXTENSION \"%s\"", old)); err != nil {
				return err
			}
		}
	}
	for _, new := range extensions {
		if new == "plpgsql" {
			// plpgsql should always be enabled
			break
		}
		found = false
		for _, old := range oldExtensions {
			if old == new {
				found = true
				break
			}
		}
		if !found {
			d.logger.Debug("create-extension", lager.Data{"extension": new})
			if _, err := d.db.Exec(fmt.Sprintf("CREATE EXTENSION \"%s\"", new)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *PostgresEngine) URI(dbname string, username string, password string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?reconnect=true", username, password, dbname, d.config.Port, d.config.DBName)
}

func (d *PostgresEngine) JDBCURI(dbname string, username string, password string) string {
	return fmt.Sprintf("jdbc:postgresql://%s:%d/%s?user=%s&password=%s", d.config.Url, d.config.Port, dbname, username, password)
}

func (d *PostgresEngine) dropConnections(dbname string) error {
	dropDBConnectionsStatement := "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '" + dbname + "' AND pid <> pg_backend_pid()"
	d.logger.Debug("drop-connections", lager.Data{"statement": dropDBConnectionsStatement})

	if _, err := d.db.Exec(dropDBConnectionsStatement); err != nil {
		d.logger.Error("sql-error", err)
		return err
	}

	return nil
}

func (d *PostgresEngine) connectionString() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user='%s' password='%s' sslmode='%s'", d.config.Url, d.config.Port, d.config.DBName, d.config.Username, d.config.Password, d.config.Sslmode)
}

func (d *PostgresEngine) Config() config.DBConfig {
	return d.config
}

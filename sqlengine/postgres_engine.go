package sqlengine

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL Driver

	"code.cloudfoundry.org/lager"
	"github.com/AusDTO/pe-rds-broker/config"
)

type PostgresEngine struct {
	logger  lager.Logger
	db      *sql.DB
	address string
	port    int64
}

func NewPostgresEngine(logger lager.Logger) *PostgresEngine {
	return &PostgresEngine{
		logger: logger.Session("postgres-engine"),
	}
}

func (d *PostgresEngine) Open(address string, port int64, dbname string, username string, password string, sslmode config.SSLMode) error {
	d.address = address
	d.port = port
	connectionString := d.connectionString(dbname, username, password, sslmode)
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

func (d *PostgresEngine) URI(dbname string, username string, password string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?reconnect=true", username, password, d.address, d.port, dbname)
}

func (d *PostgresEngine) JDBCURI(dbname string, username string, password string) string {
	return fmt.Sprintf("jdbc:postgresql://%s:%d/%s?user=%s&password=%s", d.address, d.port, dbname, username, password)
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

func (d *PostgresEngine) connectionString(dbname string, username string, password string, sslmode config.SSLMode) string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user='%s' password='%s' sslmode='%s'", d.address, d.port, dbname, username, password, sslmode)
}

func (d *PostgresEngine) Address() string {
	return d.address
}

func (d *PostgresEngine) Port() int64 {
	return d.port
}

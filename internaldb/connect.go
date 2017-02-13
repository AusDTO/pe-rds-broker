package internaldb

import (
	"fmt"
	"errors"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"code.cloudfoundry.org/lager"
	"github.com/AusDTO/pe-rds-broker/config"
)

// Supported DB types:
// * postgres
// * sqlite3
func DBInit(dbConfig *config.DBConfig, logger lager.Logger) (*gorm.DB, error) {
	var DB *gorm.DB
	var err error
	switch dbConfig.DBType {
	case "postgres":
		conn := "dbname=%s user=%s password=%s host=%s sslmode=%s port=%d"
		conn = fmt.Sprintf(conn,
			dbConfig.DBName,
			dbConfig.Username,
			dbConfig.Password,
			dbConfig.Url,
			dbConfig.Sslmode,
			dbConfig.Port)
		DB, err = gorm.Open("postgres", conn)
	case "sqlite3":
		DB, err = gorm.Open("sqlite3", dbConfig.DBName)
	default:
		err = errors.New("Cannot connect. Unsupported DB type: (" + dbConfig.DBType + ")")
		logger.Error("connectdb", err)
		return nil, err
	}
	if err != nil {
		logger.Error("connectdb", err)
		return nil, err
	}

	if err = DB.DB().Ping(); err != nil {
		logger.Error("connectdb-ping", err)
		return nil, err
	}
	migrate(DB, dbConfig, logger)
	return DB, nil
}

func migrate(db *gorm.DB, dbConfig *config.DBConfig, logger lager.Logger) {
	db.AutoMigrate(&DBInstance{}, &DBUser{}, &DBBinding{})
	// AutoMigrate does not handle FK contraints, nor does sqlite
	if dbConfig.DBType == "postgres" {
		err := db.Model(&DBUser{}).AddForeignKey(
			"db_instance_id", // instance_id field of the DBUser table
			"db_instances(id)", // references the id field of the db_instances table
			"CASCADE", // on delete CASCADE
			"RESTRICT", // on update RESTRICT
		).Error
		if err != nil {
			logger.Error("add-fk", err)
		}
		err = db.Model(&DBBinding{}).AddForeignKey(
			"db_user_id",
			"db_users(id)",
			"CASCADE",
			"RESTRICT",
		).Error
		if err != nil {
			logger.Error("add-fk", err)
		}
	}
}

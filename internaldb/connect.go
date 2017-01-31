package internaldb

import (
	"fmt"
	"log"
	"errors"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

//   Valid SSL modes:
//    * disable - No SSL
//    * require - Always SSL (skip verification)
//    * verify-full - Always SSL (require verification)
type DBConfig struct {
	DBType   string
	Url      string
	Username string
	Password string
	DBName   string
	Sslmode  string
	Port     int
}

// Supported DB types:
// * postgres
// * sqlite3
func DBInit(dbConfig *DBConfig) (*gorm.DB, error) {
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
		errorString := "Cannot connect. Unsupported DB type: (" + dbConfig.DBType + ")"
		log.Println(errorString)
		return nil, errors.New(errorString)
	}
	if err != nil {
		log.Println("Error!")
		return nil, err
	}

	if err = DB.DB().Ping(); err != nil {
		log.Println("Unable to verify connection to database")
		return nil, err
	}
	DB.AutoMigrate(&DBInstance{}, &DBUser{})
	return DB, nil
}

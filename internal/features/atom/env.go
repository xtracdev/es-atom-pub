package atom

import (
	"database/sql"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"os"
	"strings"
)

type envConfig struct {
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBSvc      string
}

func (ec *envConfig) MaskedConnectString() string {
	return fmt.Sprintf("%s/%s@//%s:%s/%s",
		ec.DBUser, "XXX", ec.DBHost, ec.DBPort, ec.DBSvc)
}

func (ec *envConfig) ConnectString() string {
	return fmt.Sprintf("%s/%s@//%s:%s/%s",
		ec.DBUser, ec.DBPassword, ec.DBHost, ec.DBPort, ec.DBSvc)
}

func NewEnvConfig() (*envConfig, error) {
	var configErrors []string

	user := os.Getenv("DB_USER")
	if user == "" {
		configErrors = append(configErrors, "Configuration missing DB_USER env variable")
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		configErrors = append(configErrors, "Configuration missing DB_PASSWORD env variable")
	}

	dbhost := os.Getenv("DB_HOST")
	if dbhost == "" {
		configErrors = append(configErrors, "Configuration missing DB_HOST env variable")
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		configErrors = append(configErrors, "Configuration missing DB_PORT env variable")
	}

	dbSvc := os.Getenv("DB_SVC")
	if dbSvc == "" {
		configErrors = append(configErrors, "Configuration missing DB_SVC env variable")
	}

	if len(configErrors) != 0 {
		return nil, errors.New(strings.Join(configErrors, "\n"))
	}

	return &envConfig{
		DBUser:     user,
		DBPassword: password,
		DBHost:     dbhost,
		DBPort:     dbPort,
		DBSvc:      dbSvc,
	}, nil

}

func initializeEnvironment() (*envConfig, *sql.DB, error) {
	env, err := NewEnvConfig()
	if err != nil {
		return nil, nil, err
	}

	log.Infof("Connection for test: %s", env.MaskedConnectString())

	db, err := sql.Open("oci8", env.ConnectString())
	if err != nil {
		return nil, nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, nil, err
	}

	return env, db, nil
}

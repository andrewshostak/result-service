package repository

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/andrewshostak/result-service/config"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func EstablishDatabaseConnection(cfg config.Config) *gorm.DB {
	connectionParams := fmt.Sprintf(
		"host=%s user=%s password=%s port=%s database=%s sslmode=disable",
		cfg.PG.Host,
		cfg.PG.User,
		cfg.PG.Password,
		cfg.PG.Port,
		cfg.PG.Database,
	)

	customLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(connectionParams), &gorm.Config{
		Logger: customLogger,
	})
	if err != nil {
		panic(err)
	}

	return db
}

package main

import (
	"errors"

	"github.com/andrewshostak/result-service/config"
	loggerinternal "github.com/andrewshostak/result-service/internal/infra/logger"
	"github.com/andrewshostak/result-service/internal/infra/postgres"
	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "migrate",
		Short: "migrate execute migration actions",
	}

	cmdMigrateUp := &cobra.Command{
		Use:   "up",
		Short: "migrate all the way up",
		RunE: func(_ *cobra.Command, _ []string) error {
			return up()
		},
	}

	cmdMigrateDown := &cobra.Command{
		Use:   "down",
		Short: "migrate all the way down",
		RunE: func(_ *cobra.Command, _ []string) error {
			return down()
		},
	}

	rootCmd.AddCommand(cmdMigrateUp)
	rootCmd.AddCommand(cmdMigrateDown)

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func up() error {
	m, l := run()

	err := m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		l.Info().Msg("database is up to date")
		return nil
	}

	if err != nil {
		return err
	}

	l.Info().Msg("migration up done")

	return nil
}

func down() error {
	m, l := run()

	err := m.Down()
	if errors.Is(err, migrate.ErrNoChange) {
		l.Info().Msg("nothing to migrate down")
		return nil
	}

	if err != nil {
		return err
	}

	l.Info().Msg("migration down done")

	return nil
}

func run() (*migrate.Migrate, *zerolog.Logger) {
	cfg := config.Parse[config.Migrate]()

	logger := loggerinternal.SetupLogger()

	db := postgres.EstablishDatabaseConnection(cfg.PG)

	sqlDb, err := db.DB()
	if err != nil {
		panic(err)
	}

	driver, err := migratepg.WithInstance(sqlDb, &migratepg.Config{})
	if err != nil {
		panic(err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://./database/migrations", cfg.PG.Database, driver)
	if err != nil {
		panic(err)
	}

	return m, logger
}

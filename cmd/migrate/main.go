package main

import (
	"errors"
	"io/fs"

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
		Short: "migrate all the way up, or N steps if --steps is provided",
		RunE: func(cmd *cobra.Command, _ []string) error {
			steps, _ := cmd.Flags().GetInt("steps")
			return up(steps)
		},
	}
	cmdMigrateUp.Flags().Int("steps", 0, "number of migrations to apply (0 means all the way up)")

	cmdMigrateDown := &cobra.Command{
		Use:   "down",
		Short: "migrate all the way down, or N steps if --steps is provided",
		RunE: func(cmd *cobra.Command, _ []string) error {
			steps, _ := cmd.Flags().GetInt("steps")
			return down(steps)
		},
	}
	cmdMigrateDown.Flags().Int("steps", 0, "number of migrations to roll back (0 means all the way down)")

	rootCmd.AddCommand(cmdMigrateUp)
	rootCmd.AddCommand(cmdMigrateDown)

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func up(steps int) error {
	m, l := run()

	var err error
	if steps > 0 {
		err = m.Steps(steps)
	} else {
		err = m.Up()
	}

	if errors.Is(err, migrate.ErrNoChange) || errors.Is(err, fs.ErrNotExist) {
		l.Info().Msg("database is up to date")
		return nil
	}

	if err != nil {
		return err
	}

	l.Info().Msg("migration up done")

	return nil
}

func down(steps int) error {
	m, l := run()

	var err error
	if steps > 0 {
		err = m.Steps(-steps)
	} else {
		err = m.Down()
	}

	if errors.Is(err, migrate.ErrNoChange) || errors.Is(err, fs.ErrNotExist) {
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

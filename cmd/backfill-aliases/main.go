package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/config"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "run",
		Short: "Backfills aliases",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}

	rootCmd.Flags().StringSlice("dates", []string{"2025-12-11"}, "query param in leagues endpoint of football-api")

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func run(cmd *cobra.Command, _ []string) {
	dates, err := cmd.Flags().GetStringSlice("dates")
	if err != nil {
		panic(err)
	}

	if len(dates) == 0 {
		panic(errors.New("dates param cannot be empty"))
	}

	parsedDates := make([]time.Time, 0, len(dates))
	for _, date := range dates {
		parsed, err := time.Parse(time.DateOnly, date)
		if err != nil {
			panic(err)
		}
		parsedDates = append(parsedDates, parsed)
	}

	cfg := config.Parse[config.BackfillAliases]()

	logger := loggerinternal.SetupLogger()

	httpClient := http.Client{}

	db := repository.EstablishDatabaseConnection(cfg.PG)

	aliasRepository := repository.NewAliasRepository(db)

	fotmobClient := client.NewFotmobClient(&httpClient, logger, cfg.ExternalAPI.FotmobAPIBaseURL)

	backfillAliasesService := service.NewBackfillAliasesService(aliasRepository, fotmobClient, logger)

	ctx := context.Background()

	err = backfillAliasesService.Backfill(ctx, parsedDates)
	if err != nil {
		panic(err)
	}
}

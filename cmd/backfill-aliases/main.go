package main

import (
	"context"
	"errors"
	"net/http"

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

	rootCmd.Flags().Uint("season", 0, "query param in leagues endpoint of football-api")

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func run(cmd *cobra.Command, _ []string) {
	season, err := cmd.Flags().GetUint("season")
	if err != nil {
		panic(err)
	}

	if season == 0 {
		panic(errors.New("season flag cannot be empty"))
	}

	cfg := config.BackfillAliases{}
	cfg.Parse()

	logger := loggerinternal.SetupLogger()

	httpClient := http.Client{}

	db := repository.EstablishDatabaseConnection(cfg.PG)

	aliasRepository := repository.NewAliasRepository(db)

	footballAPIClient := client.NewFootballAPIClient(&httpClient, logger, cfg.ExternalAPI.FootballAPIBaseURL, cfg.ExternalAPI.RapidAPIKey)

	backfillAliasesService := service.NewBackfillAliasesService(aliasRepository, footballAPIClient, logger)

	ctx := context.Background()

	err = backfillAliasesService.Backfill(ctx, season)
	if err != nil {
		panic(err)
	}
}

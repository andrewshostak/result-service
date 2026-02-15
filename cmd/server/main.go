package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/notifier"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/task"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/alias"
	"github.com/andrewshostak/result-service/internal/app/match"
	"github.com/andrewshostak/result-service/internal/app/subscription"
	"github.com/andrewshostak/result-service/internal/infra/http/server"
	loggerinternal "github.com/andrewshostak/result-service/internal/infra/logger"
	"github.com/andrewshostak/result-service/internal/infra/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "run",
		Short: "Server starts running the server",
		Run:   startServer,
	}

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func startServer(_ *cobra.Command, _ []string) {
	cfg := config.Parse[config.Server]()

	logger := loggerinternal.SetupLogger()

	db := postgres.EstablishDatabaseConnection(cfg.PG)
	httpClient := http.Client{Timeout: cfg.App.TriggersTimeout - (2 * time.Second)}

	ctx := context.Background()
	cloudTasksClient, err := cloudtasks.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	defer cloudTasksClient.Close()

	fotmobClient := fotmob.NewFotmobClient(&httpClient, logger, cfg.ExternalAPI)
	notifierClient := notifier.NewNotifierClient(&httpClient, logger)
	taskClient := task.NewClient(cfg.GoogleCloud, cfg.App.TriggersTimeout+(2*time.Second), cloudTasksClient)

	aliasRepository := repository.NewAliasRepository(db)
	matchRepository := repository.NewMatchRepository(db)
	externalMatchRepository := repository.NewExternalMatchRepository(db)
	subscriptionRepository := repository.NewSubscriptionRepository(db)
	checkResultTaskRepository := repository.NewCheckResultTaskRepository(db)

	matchService := match.NewMatchService(
		cfg.Result,
		aliasRepository,
		matchRepository,
		externalMatchRepository,
		checkResultTaskRepository,
		fotmobClient,
		taskClient,
		logger,
	)
	subscriptionService := subscription.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)
	aliasService := alias.NewAliasService(aliasRepository, logger)
	resultCheckerService := match.NewResultCheckerService(
		cfg.Result,
		matchRepository,
		externalMatchRepository,
		subscriptionRepository,
		checkResultTaskRepository,
		taskClient,
		fotmobClient,
		logger,
	)
	subscriberNotifierService := subscription.NewSubscriberNotifierService(subscriptionRepository, matchRepository, notifierClient, logger)

	r, err := server.NewServer(cfg, server.Handlers{
		MatchHandler:        handler.NewMatchHandler(matchService),
		SubscriptionHandler: handler.NewSubscriptionHandler(subscriptionService),
		AliasHandler:        handler.NewAliasHandler(aliasService),
		TriggerHandler:      handler.NewTriggerHandler(resultCheckerService, subscriberNotifierService),
	})
	if err != nil {
		panic(fmt.Errorf("failed to configure server: %w", err))
	}

	_ = r.Run(fmt.Sprintf(":%s", cfg.App.Port))
}

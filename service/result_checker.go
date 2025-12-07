package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/repository"
)

var matchFinishedStatuses = []string{"FT", "AET", "PEN"}
var matchCancellResultStatuses = []string{"TBD", "NS", "PST", "CANC", "ABD", "AWD", "WO"}
var matchInPlayStatuses = []string{"LIVE", "1H", "HT", "2H", "ET", "BT", "P", "SUSP", "INT"}

type ResultCheckerService struct {
	config                       config.ResultCheck
	matchRepository              MatchRepository
	footballAPIFixtureRepository FootballAPIFixtureRepository
	subscriptionRepository       SubscriptionRepository
	checkResultTaskRepository    CheckResultTaskRepository
	footballAPIClient            FootballAPIClient
	taskClient                   TaskClient
	logger                       Logger
}

func NewResultCheckerService(
	config config.ResultCheck,
	matchRepository MatchRepository,
	footballAPIFixtureRepository FootballAPIFixtureRepository,
	subscriptionRepository SubscriptionRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	taskClient TaskClient,
	footballAPIClient FootballAPIClient,
	logger Logger,
) *ResultCheckerService {
	return &ResultCheckerService{
		config:                       config,
		matchRepository:              matchRepository,
		footballAPIFixtureRepository: footballAPIFixtureRepository,
		subscriptionRepository:       subscriptionRepository,
		checkResultTaskRepository:    checkResultTaskRepository,
		taskClient:                   taskClient,
		footballAPIClient:            footballAPIClient,
		logger:                       logger,
	}
}

func (s *ResultCheckerService) CheckResult(ctx context.Context, matchID uint) error {
	match, err := s.matchRepository.One(ctx, repository.Match{ID: matchID})
	if err != nil {
		return fmt.Errorf("failed to get match by id: %w", err)
	}

	if !s.isScheduled(match) {
		s.logger.Error().
			Uint("match_id", matchID).
			Msg(fmt.Sprintf("expected result status to be %s, actual result status is %s", repository.Scheduled, match.ResultStatus))
		return nil
	}

	if match.FootballApiFixtures == nil || len(match.FootballApiFixtures) == 0 {
		s.logger.Error().
			Uint("match_id", matchID).
			Msg("no football api fixtures found for the match")
		return errors.New("match doesn't have any football api fixtures")
	}

	response, err := s.footballAPIClient.SearchFixtures(ctx, client.FixtureSearch{ID: &match.FootballApiFixtures[0].ID})
	if err != nil {
		s.logger.Error().Uint("match_id", matchID).Err(err)
		if errUpdate := s.updateMatchResultStatus(ctx, match.ID, repository.APIError); errUpdate != nil {
			s.logger.Error().Uint("match_id", matchID).Err(errUpdate)
		}

		return fmt.Errorf("failed to get fixture from football api: %w", err)
	}

	fixture := fromClientFootballAPIFixture(response.Response[0])
	_, err = s.footballAPIFixtureRepository.Update(ctx, match.FootballApiFixtures[0].ID, toRepositoryFootballAPIFixtureData(fixture))
	if err != nil {
		s.logger.Error().
			Uint("match_id", matchID).
			Err(err)
		return fmt.Errorf("failed to update fixture: %w", err)
	}

	if slices.Contains(matchCancellResultStatuses, fixture.Fixture.Status.Short) {
		return s.handleCancelledFixture(ctx, matchID)
	}

	if slices.Contains(matchInPlayStatuses, fixture.Fixture.Status.Short) {
		return s.handleInPlayFixture(ctx, *match)
	}

	if slices.Contains(matchFinishedStatuses, fixture.Fixture.Status.Short) {
		return s.handleFinishedFixture(ctx, match.ID, fixture)
	}

	return errors.New(fmt.Sprintf("unexpected fixture status received: %s - %s", fixture.Fixture.Status.Short, fixture.Fixture.Status.Long))
}

func (s *ResultCheckerService) handleCancelledFixture(ctx context.Context, matchID uint) error {
	if err := s.updateMatchResultStatus(ctx, matchID, repository.Cancelled); err != nil {
		s.logger.Error().Uint("match_id", matchID).Err(err)
		return fmt.Errorf("failed to handle cancelled fixture: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleInPlayFixture(ctx context.Context, match repository.Match) error {
	if match.CheckResultTask == nil {
		return errors.New("match doesn't have a result check task")
	}

	scheduleAt := match.StartsAt.Add(s.config.FirstAttemptDelay)
	for i := uint(0); i <= match.CheckResultTask.AttemptNumber; i++ {
		scheduleAt.Add(s.config.Interval)
	}

	name, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, match.CheckResultTask.AttemptNumber, scheduleAt)
	if err != nil {
		if errUpdate := s.updateMatchResultStatus(ctx, match.ID, repository.SchedulingError); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate)
		}

		return fmt.Errorf("failed to re-schedule result check task: %w", err)
	}

	if _, err := s.checkResultTaskRepository.Update(ctx, match.CheckResultTask.ID, repository.CheckResultTask{
		Name:          *name,
		AttemptNumber: match.CheckResultTask.AttemptNumber + 1,
	}); err != nil {
		return fmt.Errorf("failed to update result check task: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleFinishedFixture(ctx context.Context, matchID uint, fixture Data) error {
	if err := s.updateMatchResultStatus(ctx, matchID, repository.Received); err != nil {
		return fmt.Errorf("failed to handle finished fixture: %w", err)
	}

	subscriptions, err := s.subscriptionRepository.ListPending(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		s.logger.Error().Uint("match_id", matchID).Msg("no pending subscriptions found for the match")
	}

	for _, subscription := range subscriptions {
		err := s.taskClient.ScheduleSubscriberNotification(ctx, subscription.ID)
		if err != nil {
			errUpdate := s.subscriptionRepository.Update(ctx, subscription.ID, repository.Subscription{Status: repository.SchedulingErrorSub})
			if errUpdate != nil {
				s.logger.Error().Err(errUpdate).Uint("subscription_id", subscription.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", repository.SchedulingErrorSub))
			}

			return fmt.Errorf("failed to schedule subscriber notification: %w", err)
		}
	}

	return nil
}

func (s *ResultCheckerService) updateMatchResultStatus(ctx context.Context, matchID uint, status repository.ResultStatus) error {
	if _, errUpdate := s.matchRepository.Update(ctx, matchID, status); errUpdate != nil {
		return fmt.Errorf("failed to set result status to %s: %w", status, errUpdate)
	}

	return nil
}

func (s *ResultCheckerService) isScheduled(match *repository.Match) bool {
	return match != nil && match.ResultStatus == repository.Scheduled
}

package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
)

type MatchService struct {
	config                    config.ResultCheck
	aliasRepository           AliasRepository
	matchRepository           MatchRepository
	externalMatchRepository   ExternalMatchRepository
	checkResultTaskRepository CheckResultTaskRepository
	externalAPIClient         ExternalAPIClient
	taskClient                TaskClient
	logger                    Logger
}

func NewMatchService(
	config config.ResultCheck,
	aliasRepository AliasRepository,
	matchRepository MatchRepository,
	externalMatchRepository ExternalMatchRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	externalAPIClient ExternalAPIClient,
	taskClient TaskClient,
	logger Logger,
) *MatchService {
	return &MatchService{
		config:                    config,
		aliasRepository:           aliasRepository,
		matchRepository:           matchRepository,
		externalMatchRepository:   externalMatchRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		externalAPIClient:         externalAPIClient,
		taskClient:                taskClient,
		logger:                    logger,
	}
}

func (s *MatchService) Create(ctx context.Context, request models.CreateMatchRequest) (uint, error) {
	if request.StartsAt.Before(time.Now()) {
		return 0, errs.NewUnprocessableContentError(errors.New("match starting time must be in the future"))
	}

	aliasHome, err := s.findAlias(ctx, request.AliasHome)
	if err != nil {
		return 0, fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.findAlias(ctx, request.AliasAway)
	if err != nil {
		return 0, fmt.Errorf("failed to find away team alias: %w", err)
	}

	match, errMatch := s.matchRepository.One(ctx, models.Match{
		StartsAt: request.StartsAt.UTC(),
		HomeTeam: &models.Team{ID: aliasHome.TeamID},
		AwayTeam: &models.Team{ID: aliasAway.TeamID},
	})
	if errMatch != nil && !errors.As(errMatch, &errs.ResourceNotFoundError{}) {
		return 0, fmt.Errorf("failed to find match: %w", errMatch)
	}

	if match != nil {
		if s.isResultCheckScheduled(*match) {
			return match.ID, nil
		}

		if !s.isResultCheckNotScheduled(*match) {
			return 0, errs.NewUnprocessableContentError(errors.New(fmt.Sprintf("match already exists with result status: %s", match.ResultStatus)))
		}
	}

	leagues, err := s.externalAPIClient.GetMatchesByDate(ctx, request.StartsAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("failed to get matches from external api: %w", err)
	}

	externalMatch, err := s.findExternalMatch(aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, leagues)
	if err != nil {
		return 0, errs.NewUnprocessableContentError(errors.New(fmt.Sprintf("external match with home team id %d and away team id %d is not found: %s", aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, err.Error())))
	}

	if !s.isResultCheckSchedulingAllowed(*externalMatch) {
		return 0, errs.NewUnprocessableContentError(errors.New(fmt.Sprintf("result check scheduling is not allowed for this match, external match status is %s", externalMatch.Status)))
	}

	var matchID *uint
	if match != nil {
		matchID = &match.ID
	}

	match, err = s.matchRepository.Save(ctx, matchID, models.Match{
		HomeTeam:     &models.Team{ID: aliasHome.TeamID},
		AwayTeam:     &models.Team{ID: aliasAway.TeamID},
		StartsAt:     externalMatch.Time,
		ResultStatus: models.NotScheduled,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, externalMatch.Time, err)
	}

	externalMatchID := uint(externalMatch.ID)
	_, err = s.externalMatchRepository.Save(ctx, &externalMatchID, externalMatch.ToExternalMatch(match.ID))
	if err != nil {
		return 0, fmt.Errorf("failed to save external match with id %d and match id %d: %w", externalMatchID, match.ID, err)
	}

	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, 1, match.StartsAt.Add(s.config.FirstAttemptDelay))
	if err != nil && !errors.As(err, &errs.ResourceAlreadyExistsError{}) {
		return 0, fmt.Errorf("failed to schedule result check task: %w", err)
	}

	if errors.As(err, &errs.ResourceAlreadyExistsError{}) {
		foundTask, errGetTask := s.taskClient.GetResultCheckTask(ctx, match.ID, 1)
		if errGetTask != nil {
			return 0, fmt.Errorf("failed to get result check task: %w", errGetTask)
		}
		task = foundTask
	}

	_, err = s.checkResultTaskRepository.Save(ctx, models.CheckResultTask{
		MatchID:       match.ID,
		Name:          task.Name,
		AttemptNumber: 1,
		ExecuteAt:     task.ExecuteAt,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to save result-check task: %w", err)
	}

	s.logger.Debug().Uint("match_id", match.ID).Time("execute_at", task.ExecuteAt).Msg("match saved & check result task scheduled")

	_, err = s.matchRepository.Update(ctx, match.ID, models.Scheduled)
	if err != nil {
		return 0, fmt.Errorf("failed to update match status to %s: %w", models.Scheduled, err)
	}

	return match.ID, nil
}

func (s *MatchService) findAlias(ctx context.Context, alias string) (*models.Alias, error) {
	foundAlias, err := s.aliasRepository.Find(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to find team alias: %w", err)
	}

	if foundAlias.ExternalTeam == nil {
		return nil, errors.New(fmt.Sprintf("alias %s doesn't have external team relation", alias))
	}

	return foundAlias, nil
}

func (s *MatchService) findExternalMatch(externalHomeTeamID, externalAwayTeamID uint, leagues []models.ExternalAPILeague) (*models.ExternalAPIMatch, error) {
	for _, matches := range leagues {
		for _, match := range matches.Matches {
			if match.Home.ID == int(externalHomeTeamID) && match.Away.ID == int(externalAwayTeamID) {
				return &match, nil
			}
		}
	}

	return nil, errors.New("match not found")
}

func (s *MatchService) isResultCheckSchedulingAllowed(externalMatch models.ExternalAPIMatch) bool {
	return externalMatch.Status == models.StatusMatchNotStarted || externalMatch.Status == models.StatusMatchInProgress
}

func (s *MatchService) isResultCheckScheduled(match models.Match) bool {
	return match.ResultStatus == models.Scheduled
}

func (s *MatchService) isResultCheckNotScheduled(match models.Match) bool {
	return match.ResultStatus == models.NotScheduled
}

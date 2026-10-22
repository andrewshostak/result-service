package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/models"
)

type MatchService struct {
	config                    config.ResultCheck
	aliasRepository           AliasRepository
	matchRepository           MatchRepository
	providerTeamRepository    ProviderTeamRepository
	providerMatchRepository   ProviderMatchRepository
	checkResultTaskRepository CheckResultTaskRepository
	providers                 []ResultProvider
	taskClient                TaskClient
	logger                    Logger
}

func NewMatchService(
	config config.ResultCheck,
	aliasRepository AliasRepository,
	matchRepository MatchRepository,
	providerTeamRepository ProviderTeamRepository,
	providerMatchRepository ProviderMatchRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	providers []ResultProvider,
	taskClient TaskClient,
	logger Logger,
) *MatchService {
	return &MatchService{
		config:                    config,
		aliasRepository:           aliasRepository,
		matchRepository:           matchRepository,
		providerTeamRepository:    providerTeamRepository,
		providerMatchRepository:   providerMatchRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		providers:                 providers,
		taskClient:                taskClient,
		logger:                    logger,
	}
}

func (s *MatchService) Create(ctx context.Context, request models.CreateMatchRequest) (uint, error) {
	aliasHome, err := s.findAlias(ctx, request.AliasHome)
	if err != nil {
		return 0, fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.findAlias(ctx, request.AliasAway)
	if err != nil {
		return 0, fmt.Errorf("failed to find away team alias: %w", err)
	}

	match, errMatch := s.matchRepository.One(ctx, models.Match{
		StartsAt:   request.StartsAt,
		HomeTeamID: aliasHome.TeamID,
		AwayTeamID: aliasAway.TeamID,
	})
	if errMatch != nil && !errors.As(errMatch, &models.ResourceNotFoundError{}) {
		return 0, fmt.Errorf("failed to find match: %w", errMatch)
	}

	if match != nil {
		if s.isResultCheckScheduled(*match) {
			return match.ID, nil
		}

		if !s.isResultCheckNotScheduled(*match) {
			return 0, models.NewUnprocessableContentError(errors.New(fmt.Sprintf("match already exists with result status: %s", match.ResultStatus)))
		}
	}

	providerTeamHome, err := s.providerTeamRepository.FindByTeamAndProvider(ctx, aliasHome.TeamID, fotmobProvider)
	if err != nil {
		return 0, fmt.Errorf("provider team for home team id %d not found: %w", aliasHome.TeamID, err)
	}

	providerTeamAway, err := s.providerTeamRepository.FindByTeamAndProvider(ctx, aliasAway.TeamID, fotmobProvider)
	if err != nil {
		return 0, fmt.Errorf("provider team for away team id %d not found: %w", aliasAway.TeamID, err)
	}

	providersMatches, err := s.getMatchesFromProviders(ctx, request.StartsAt)
	if err != nil {
		return 0, fmt.Errorf("failed to get matches from external api: %w", err)
	}

	externalMatch, providerName, err := s.findExternalMatch(providerTeamHome.ProviderTeamID, providerTeamAway.ProviderTeamID, providersMatches)
	if err != nil {
		return 0, models.NewUnprocessableContentError(errors.New(fmt.Sprintf("external match with home provider team id %s and away provider team id %s is not found: %s", providerTeamHome.ProviderTeamID, providerTeamAway.ProviderTeamID, err.Error())))
	}

	if !s.isResultCheckSchedulingAllowed(*externalMatch) {
		return 0, models.NewUnprocessableContentError(errors.New(fmt.Sprintf("result check scheduling is not allowed for this match, external match status is %s", externalMatch.Status)))
	}

	var matchID *uint
	if match != nil {
		matchID = &match.ID
	}

	match, err = s.matchRepository.Save(ctx, matchID, models.Match{
		HomeTeamID:   aliasHome.TeamID,
		AwayTeamID:   aliasAway.TeamID,
		StartsAt:     externalMatch.Time,
		ResultStatus: models.NotScheduled,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, externalMatch.Time, err)
	}

	_, err = s.providerMatchRepository.Save(ctx, externalMatch.ToProviderMatch(match.ID, *providerName))
	if err != nil {
		return 0, fmt.Errorf("failed to save provider match with id %d and match id %d: %w", externalMatch.ID, match.ID, err)
	}

	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, 1, match.StartsAt.Add(s.config.FirstAttemptDelay))
	if err != nil && !errors.As(err, &models.ResourceAlreadyExistsError{}) {
		return 0, fmt.Errorf("failed to schedule result check task: %w", err)
	}

	if errors.As(err, &models.ResourceAlreadyExistsError{}) {
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

	_, err = s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.Scheduled})
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

	return foundAlias, nil
}

func (s *MatchService) findExternalMatch(homeProviderTeamID, awayProviderTeamID string, providersMatches map[string][]models.ExternalAPIMatch) (*models.ExternalAPIMatch, *string, error) {
	for provider, matches := range providersMatches {
		for _, match := range matches {
			if match.HomeID == homeProviderTeamID && match.AwayID == awayProviderTeamID {
				return &match, &provider, nil
			}
		}
	}

	return nil, nil, errors.New("match not found")
}

func (s *MatchService) getMatchesFromProviders(ctx context.Context, startsAt time.Time) (map[string][]models.ExternalAPIMatch, error) {
	var matchesErrors []error
	matches := make(map[string][]models.ExternalAPIMatch)
	for _, provider := range s.providers {
		// parallelize in case of multiple providers
		providerMatches, err := provider.GetMatches(ctx, startsAt)
		if err != nil {
			matchesErrors = append(matchesErrors, fmt.Errorf("failed to get matches from provider %s: %w", provider.Name(), err))
			continue
		}

		matches[provider.Name()] = providerMatches
	}

	if len(matches) != 0 {
		return matches, nil
	}

	return nil, fmt.Errorf("failed to get matches across providers: %w", errors.Join(matchesErrors...))
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

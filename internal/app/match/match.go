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
		return 0, fmt.Errorf("failed to get matches from provider api: %w", err)
	}

	providerAPIMatch, providerName, err := s.findProviderAPIMatch(providerTeamHome.ProviderTeamID, providerTeamAway.ProviderTeamID, providersMatches)
	if err != nil {
		return 0, models.NewUnprocessableContentError(errors.New(fmt.Sprintf("provider match with home provider team id %s and away provider team id %s is not found: %s", providerTeamHome.ProviderTeamID, providerTeamAway.ProviderTeamID, err.Error())))
	}

	if !s.isResultCheckSchedulingAllowed(*providerAPIMatch) {
		return 0, models.NewUnprocessableContentError(errors.New(fmt.Sprintf("result check scheduling is not allowed for this match, provider match status is %s", providerAPIMatch.Status)))
	}

	var matchID *uint
	if match != nil {
		matchID = &match.ID
	}

	match, err = s.matchRepository.Save(ctx, matchID, models.Match{
		HomeTeamID:   aliasHome.TeamID,
		AwayTeamID:   aliasAway.TeamID,
		StartsAt:     providerAPIMatch.Time,
		ResultStatus: models.NotScheduled,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, providerAPIMatch.Time, err)
	}

	_, err = s.providerMatchRepository.Save(ctx, providerAPIMatch.ToProviderMatch(match.ID, *providerName))
	if err != nil {
		return 0, fmt.Errorf("failed to save provider match with id %d and match id %d: %w", providerAPIMatch.ID, match.ID, err)
	}

	firstCheckBaseTime := earliestOf(request.StartsAt, match.StartsAt)
	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, 1, firstCheckBaseTime.Add(s.config.FirstAttemptDelay))
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

func (s *MatchService) findProviderAPIMatch(homeProviderTeamID, awayProviderTeamID string, providersMatches map[string][]models.ProviderAPIMatch) (*models.ProviderAPIMatch, *string, error) {
	for provider, matches := range providersMatches {
		for _, match := range matches {
			if match.HomeID == homeProviderTeamID && match.AwayID == awayProviderTeamID {
				return &match, &provider, nil
			}
		}
	}

	return nil, nil, errors.New("match not found")
}

func (s *MatchService) getMatchesFromProviders(ctx context.Context, startsAt time.Time) (map[string][]models.ProviderAPIMatch, error) {
	var matchesErrors []error
	matches := make(map[string][]models.ProviderAPIMatch)
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

func (s *MatchService) isResultCheckSchedulingAllowed(providerAPIMatch models.ProviderAPIMatch) bool {
	return providerAPIMatch.Status == models.StatusMatchNotStarted || providerAPIMatch.Status == models.StatusMatchInProgress
}

func (s *MatchService) isResultCheckScheduled(match models.Match) bool {
	return match.ResultStatus == models.Scheduled
}

func (s *MatchService) isResultCheckNotScheduled(match models.Match) bool {
	return match.ResultStatus == models.NotScheduled
}

// earliestOf returns the earlier of two timestamps. The first result-check is scheduled off
// the earlier of the admin-submitted and provider-reported kickoff time: a too-early check is
// harmless (handleInPlayMatch already retries matches that haven't finished), but a too-late
// first check is not self-correcting.
func earliestOf(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

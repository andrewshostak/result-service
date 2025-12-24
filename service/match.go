package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/repository"
)

type MatchService struct {
	config                    config.ResultCheck
	aliasRepository           AliasRepository
	matchRepository           MatchRepository
	externalMatchRepository   ExternalMatchRepository
	checkResultTaskRepository CheckResultTaskRepository
	fotmobClient              FotmobClient
	taskClient                TaskClient
	logger                    Logger
}

func NewMatchService(
	config config.ResultCheck,
	aliasRepository AliasRepository,
	matchRepository MatchRepository,
	externalMatchRepository ExternalMatchRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	fotmobClient FotmobClient,
	taskClient TaskClient,
	logger Logger,
) *MatchService {
	return &MatchService{
		config:                    config,
		aliasRepository:           aliasRepository,
		matchRepository:           matchRepository,
		externalMatchRepository:   externalMatchRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		fotmobClient:              fotmobClient,
		taskClient:                taskClient,
		logger:                    logger,
	}
}

func (s *MatchService) Create(ctx context.Context, request CreateMatchRequest) (uint, error) {
	if request.StartsAt.Before(time.Now()) {
		return 0, errors.New("match starting time must be in the future")
	}

	aliasHome, err := s.findAlias(ctx, request.AliasHome)
	if err != nil {
		return 0, fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.findAlias(ctx, request.AliasAway)
	if err != nil {
		return 0, fmt.Errorf("failed to find away team alias: %w", err)
	}

	m, errMatch := s.matchRepository.One(ctx, repository.Match{
		StartsAt:   request.StartsAt.UTC(),
		HomeTeamID: aliasHome.TeamID,
		AwayTeamID: aliasAway.TeamID,
	})
	if errMatch != nil && !errors.As(errMatch, &errs.MatchNotFoundError{}) {
		return 0, fmt.Errorf("unexpected error when getting a match: %w", errMatch)
	}

	var match *Match
	if m != nil {
		match = fromRepositoryMatch(*m)

		if s.isResultCheckScheduled(*match) {
			return match.ID, nil
		}

		if !s.isResultCheckNotScheduled(*match) {
			return 0, fmt.Errorf("match already exists with result status: %s", match.ResultStatus)
		}
	}

	matchesByDate, err := s.fotmobClient.GetMatchesByDate(ctx, request.StartsAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("failed to get matches from external api: %w", err)
	}

	leagues, err := fromClientFotmobLeagues(*matchesByDate)
	if err != nil {
		return 0, fmt.Errorf("failed to map from external api matches: %w", err)
	}

	externalMatch, err := s.findExternalMatch(aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, leagues)
	if err != nil {
		return 0, fmt.Errorf("match with home team id %d and away team id %d is not found: %w", aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, err)
	}

	if !s.isResultCheckSchedulingAllowed(*externalMatch) {
		return 0, fmt.Errorf("result check scheduling is not allowed for this match, external match status is %s", externalMatch.Status)
	}

	var matchID *uint
	if match != nil {
		matchID = &match.ID
	}
	matchToSave := toRepositoryMatch(aliasHome.TeamID, aliasAway.TeamID, externalMatch.Time, NotScheduled)
	savedMatch, err := s.matchRepository.Save(ctx, matchID, matchToSave)
	if err != nil {
		return 0, fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, externalMatch.Time, err)
	}

	s.logger.Info().Uint("match_id", savedMatch.ID).Msg("match saved")

	externalMatchID := uint(externalMatch.ID)
	savedExternalMatch, err := s.externalMatchRepository.Save(ctx, &externalMatchID, toRepositoryExternalMatch(externalMatchID, *externalMatch))
	if err != nil {
		return 0, fmt.Errorf("failed to save external match with id %d and match id %d: %w", externalMatchID, savedMatch.ID, err)
	}

	s.logger.Info().Uint("match_id", savedMatch.ID).Msg("external match saved")

	match = fromRepositoryMatch(*savedMatch)

	checkResultTask, err := s.checkResultTaskRepository.GetByMatchID(ctx, match.ID)
	if err != nil && !errors.As(err, &errs.CheckResultNotFoundError{}) {
		return 0, fmt.Errorf("failed to get check result task: %w", err)
	}

	if checkResultTask != nil {
		return match.ID, nil
	}

	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, 1, match.StartsAt.Add(s.config.FirstAttemptDelay))
	if err != nil {
		return 0, fmt.Errorf("failed to schedule result check task: %w", err)
	}

	scheduledTask := fromClientTask(*task)
	_, err = s.checkResultTaskRepository.Create(ctx, toRepositoryCheckResultTask(match.ID, scheduledTask))
	if err != nil && !errors.As(err, &errs.CheckResultTaskAlreadyExistsError{}) {
		return 0, fmt.Errorf("failed to create result-check task: %w", err)
	}

	s.logger.Info().
		Uint("match_id", match.ID).
		Uint("external_match_id", savedExternalMatch.ID).
		Str("alias_home", aliasHome.Alias).
		Str("alias_away", aliasAway.Alias).
		Msg("match result acquiring scheduled")

	_, err = s.matchRepository.Update(ctx, savedMatch.ID, string(Scheduled))
	if err != nil {
		return 0, fmt.Errorf("failed to set match status to %s: %w", Scheduled, err)
	}

	return savedMatch.ID, nil
}

func (s *MatchService) findAlias(ctx context.Context, alias string) (*Alias, error) {
	foundAlias, err := s.aliasRepository.Find(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to find team alias: %w", err)
	}

	if foundAlias.ExternalTeam == nil {
		return nil, errors.New(fmt.Sprintf("alias %s doesn't have external team relation", alias))
	}

	mapped := fromRepositoryAlias(*foundAlias)
	return &mapped, nil
}

func (s *MatchService) findExternalMatch(externalHomeTeamID, externalAwayTeamID uint, leagues []ExternalLeague) (*ExternalMatch, error) {
	for _, matches := range leagues {
		for _, match := range matches.Matches {
			if match.Home.ID == int(externalHomeTeamID) && match.Away.ID == int(externalAwayTeamID) {
				return &match, nil
			}
		}
	}

	return nil, errors.New("match not found")
}

func (s *MatchService) isResultCheckSchedulingAllowed(externalMatch ExternalMatch) bool {
	return externalMatch.Status != StatusMatchNotStarted && externalMatch.Status != StatusMatchInProgress
}

func (s *MatchService) isResultCheckScheduled(match Match) bool {
	return match.ResultStatus == Scheduled
}

func (s *MatchService) isResultCheckNotScheduled(match Match) bool {
	return match.ResultStatus == NotScheduled
}

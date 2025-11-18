package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/repository"
	"github.com/rs/zerolog"
)

const dateFormat = "2006-01-02"
const stateMatchFinished = "Match Finished"

type MatchService struct {
	aliasRepository              AliasRepository
	matchRepository              MatchRepository
	footballAPIFixtureRepository FootballAPIFixtureRepository
	footballAPIClient            FootballAPIClient
	logger                       Logger
	pollingMaxRetries            uint
	pollingInterval              time.Duration
	pollingFirstAttemptDelay     time.Duration
}

func NewMatchService(
	aliasRepository AliasRepository,
	matchRepository MatchRepository,
	footballAPIFixtureRepository FootballAPIFixtureRepository,
	footballAPIClient FootballAPIClient,
	logger Logger,
	pollingMaxRetries uint,
	pollingInterval time.Duration,
	pollingFirstAttemptDelay time.Duration,
) *MatchService {
	return &MatchService{
		aliasRepository:              aliasRepository,
		matchRepository:              matchRepository,
		footballAPIFixtureRepository: footballAPIFixtureRepository,
		footballAPIClient:            footballAPIClient,
		logger:                       logger,
		pollingMaxRetries:            pollingMaxRetries,
		pollingInterval:              pollingInterval,
		pollingFirstAttemptDelay:     pollingFirstAttemptDelay,
	}
}

func (s *MatchService) Create(ctx context.Context, request CreateMatchRequest) (uint, error) {
	aliasHome, err := s.findAlias(ctx, request.AliasHome)
	if err != nil {
		return 0, fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.findAlias(ctx, request.AliasAway)
	if err != nil {
		return 0, fmt.Errorf("failed to find away team alias: %w", err)
	}

	match, err := s.matchRepository.One(ctx, repository.Match{
		StartsAt:   request.StartsAt.UTC(),
		HomeTeamID: aliasHome.TeamID,
		AwayTeamID: aliasAway.TeamID,
	})

	// if match already exists, we can return id immediately and don't call external api
	if match != nil {
		return match.ID, nil
	}

	if !errors.As(err, &errs.MatchNotFoundError{}) {
		return 0, fmt.Errorf("unexpected error when getting a match: %w", err)
	}

	s.logger.Info().Str("alias_home", request.AliasHome).Str("alias_away", request.AliasAway).
		Msg("match is not found in the database. making an attempt to find it in external api")

	date := request.StartsAt.UTC().Format(dateFormat)
	season := uint(s.getSeason(request.StartsAt.UTC()))
	response, err := s.footballAPIClient.SearchFixtures(ctx, client.FixtureSearch{
		Season:   &season,
		Timezone: time.UTC.String(),
		Date:     &date,
		TeamID:   &aliasHome.FootballApiTeam.ID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to search fixtures in external api: %w", err)
	}

	if len(response.Response) < 1 {
		return 0, errs.UnexpectedNumberOfItemsError{Message: fmt.Sprintf("fixture starting at %s with team id %d is not found in external api", date, aliasHome.FootballApiTeam.ID)}
	}

	fixture := fromClientFootballAPIFixture(response.Response[0])

	if fixture.Fixture.Status.Long == stateMatchFinished {
		return 0, fmt.Errorf("%s: %w", fmt.Sprintf("status of the fixture with external id %d is %s", fixture.Fixture.ID, stateMatchFinished), errs.ErrIncorrectFixtureStatus)
	}

	startsAt, err := time.Parse(time.RFC3339, fixture.Fixture.Date)
	if err != nil {
		return 0, fmt.Errorf("unable to parse received from external api fixture date %s: %w", fixture.Fixture.Date, err)
	}

	toCreate := repository.Match{HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID, StartsAt: startsAt}
	created, err := s.matchRepository.Create(ctx, toCreate)
	if err != nil {
		return 0, fmt.Errorf("failed to create match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, startsAt, err)
	}

	s.logger.Info().Uint("match_id", created.ID).Msg("match saved")

	createdFixture, err := s.footballAPIFixtureRepository.Create(ctx, repository.FootballApiFixture{
		ID:      fixture.Fixture.ID,
		MatchID: created.ID,
	}, toRepositoryFootballAPIFixtureData(fixture))
	if err != nil {
		return 0, fmt.Errorf("failed to create football api fixture with match id %d: %w", created.ID, err)
	}

	s.logger.Info().Uint("football_api_fixture_id", createdFixture.ID).Uint("match_id", created.ID).Msg("fixture saved")

	mappedMatch, err := fromRepositoryMatch(*created)
	if err != nil {
		return 0, fmt.Errorf("failed to map from repository match: %w", err)
	}

	mappedFixture, err := fromRepositoryFootballAPIFixture(*createdFixture)
	if err != nil {
		return 0, fmt.Errorf("failed to map from repository api fixture: %w", err)
	}

	if err := s.scheduleMatchResultAcquiring(matchResultTaskParams{
		match:     *mappedMatch,
		fixture:   *mappedFixture,
		aliasHome: *aliasHome,
		aliasAway: *aliasAway,
		season:    season,
	}); err != nil {
		return 0, fmt.Errorf("failed to schedule match result aquiring: %w", err)
	}

	s.logger.Info().
		Uint("match_id", mappedMatch.ID).
		Uint("football_api_fixture_id", mappedFixture.ID).
		Str("alias_home", aliasHome.Alias).
		Str("alias_away", aliasAway.Alias).
		Msg("match result acquiring scheduled")

	_, err = s.matchRepository.Update(ctx, created.ID, repository.Scheduled)
	if err != nil {
		return 0, fmt.Errorf("failed to set match status to %s: %w", repository.Scheduled, err)
	}

	return created.ID, nil
}

func (s *MatchService) List(ctx context.Context, status string) ([]Match, error) {
	resultStatus := repository.ResultStatus(status)
	matches, err := s.matchRepository.List(ctx, resultStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to list matches with %s result status: %w", resultStatus, err)
	}

	mapped, err := fromRepositoryMatches(matches)
	if err != nil {
		return nil, fmt.Errorf("failed to map from repository matches: %w", err)
	}

	return mapped, nil
}

func (s *MatchService) ScheduleMatchResultAcquiring(match Match) error {
	if len(match.FootballApiFixtures) < 1 {
		return errors.New("match relation football api fixtures are not found")
	}

	if match.HomeTeam == nil || match.AwayTeam == nil {
		return errors.New("match relations home/away team are not found")
	}

	if len(match.HomeTeam.Aliases) < 1 {
		return errors.New("match relation home team doesn't have aliases")
	}

	if len(match.AwayTeam.Aliases) < 1 {
		return errors.New("match relation away team doesn't have aliases")
	}

	params := matchResultTaskParams{
		match:     Match{ID: match.ID, StartsAt: match.StartsAt},
		fixture:   match.FootballApiFixtures[0],
		aliasHome: match.HomeTeam.Aliases[0],
		aliasAway: match.AwayTeam.Aliases[0],
	}
	return s.scheduleMatchResultAcquiring(params)
}

func (s *MatchService) Update(ctx context.Context, id uint, status string) error {
	resultStatus := repository.ResultStatus(status)
	_, err := s.matchRepository.Update(ctx, id, resultStatus)
	if err != nil {
		return fmt.Errorf("failed to set match status to %s: %w", repository.Scheduled, err)
	}

	return nil
}

func (s *MatchService) findAlias(ctx context.Context, alias string) (*Alias, error) {
	foundAlias, err := s.aliasRepository.Find(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to find team alias: %w", err)
	}

	if foundAlias.FootballApiTeam == nil {
		return nil, errors.New(fmt.Sprintf("alias %s found, but there is no releated external(football api) team", alias))
	}

	mapped := fromRepositoryAlias(*foundAlias)
	return &mapped, nil
}

func (s *MatchService) scheduleMatchResultAcquiring(params matchResultTaskParams) error {
	fields := matchLogFields{
		matchID:   params.match.ID,
		aliasHome: params.aliasHome.Alias,
		aliasAway: params.aliasAway.Alias,
		startsAt:  params.match.StartsAt,
	}

	enrichLogWithMatchDetails(s.logger.Info(), fields).Msg("scheduling a task to acquire match result")

	return nil
}

// getSeason returns current year if current time is after June 3, otherwise previous year
func (s *MatchService) getSeason(startsAt time.Time) int {
	seasonBound := time.Date(startsAt.Year(), 6, 3, 0, 0, 0, 0, time.UTC)

	if startsAt.After(seasonBound) {
		return startsAt.Year()
	}

	return startsAt.AddDate(-1, 0, 0).Year()
}

func (s *MatchService) getTaskFunc(i int, ch chan<- resultTaskChan, search client.FixtureSearch, matchDetails matchLogFields) func(c context.Context) {
	return func(c context.Context) {
		enrichLogWithMatchDetails(s.logger.Info(), matchDetails).Msg(fmt.Sprintf("making an attempt %d to get match result", i))

		response, err := s.footballAPIClient.SearchFixtures(c, search)
		if err != nil {
			enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Err(err).Msg("received error when searching fixtures for match. cancelling")
			i++

			if s.retriesLimitReached(i) {
				s.writeError(matchDetails, ch)
			}
			return
		}

		if len(response.Response) < 1 {
			enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Msg("unexpected length of fixture search result")
			i++

			if s.retriesLimitReached(i) {
				s.writeError(matchDetails, ch)
			}
			return
		}

		fixture := fromClientFootballAPIFixture(response.Response[0])

		if fixture.Fixture.Status.Long != stateMatchFinished {
			enrichLogWithMatchDetails(s.logger.Info(), matchDetails).Str("status", fixture.Fixture.Status.Long).
				Msg("match status is not finished")
			i++

			if s.retriesLimitReached(i) {
				s.writeError(matchDetails, ch)
			}
			return
		}

		enrichLogWithMatchDetails(s.logger.Info(), matchDetails).
			Uint("home", fixture.Goals.Home).
			Uint("away", fixture.Goals.Away).
			Msg("match result received successfully. cancelling the task")
		ch <- resultTaskChan{fixture: &fixture}
		close(ch)
	}
}

func (s *MatchService) handleTaskResult(
	ctx context.Context,
	ch <-chan resultTaskChan,
	fixtureID uint,
	matchID uint,
	matchDetails matchLogFields,
) {
	result := <-ch

	enrichLogWithMatchDetails(s.logger.Info(), matchDetails).Msg("scheduled task cancelled")

	if result.error != nil {
		_, err := s.matchRepository.Update(ctx, matchID, repository.Error)
		if err != nil {
			enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Err(err).
				Str("result_status", string(repository.Error)).
				Msg("failed to update result status")
			return
		}

		return
	}

	if _, err := s.footballAPIFixtureRepository.Update(ctx, fixtureID, toRepositoryFootballAPIFixtureData(*result.fixture)); err != nil {
		enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Err(err).Msg("failed to update fixture")
		return
	}

	if _, err := s.matchRepository.Update(ctx, matchID, repository.Successful); err != nil {
		enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Err(err).
			Str("result_status", string(repository.Successful)).
			Msg("failed to update result status")
		return
	}

	return
}

func (s *MatchService) retriesLimitReached(i int) bool {
	return i > int(s.pollingMaxRetries)
}

func (s *MatchService) writeError(matchDetails matchLogFields, ch chan<- resultTaskChan) {
	errMessage := fmt.Sprintf("retries limit reached. cancelling")
	enrichLogWithMatchDetails(s.logger.Error(), matchDetails).Uint("retries_limit", s.pollingMaxRetries).Msg(errMessage)
	ch <- resultTaskChan{error: errors.New(errMessage)}
}

func getTaskKey(matchID uint, fixtureID uint) string {
	return fmt.Sprintf("%d-%d", matchID, fixtureID)
}

func enrichLogWithMatchDetails(event *zerolog.Event, fields matchLogFields) *zerolog.Event {
	return event.Uint("match_id", fields.matchID).
		Str("alias_home", fields.aliasHome).
		Str("alias_away", fields.aliasAway).
		Str("starts_at", fields.startsAt.String())
}

type matchLogFields struct {
	matchID   uint
	aliasHome string
	aliasAway string
	startsAt  time.Time
}

type matchResultTaskParams struct {
	match     Match
	fixture   FootballAPIFixture
	aliasHome Alias
	aliasAway Alias
	season    uint
}

type resultTaskChan struct {
	fixture *Data
	error   error
}

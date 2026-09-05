package alias

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
)

type BackfillAliasesService struct {
	aliasRepository        AliasRepository
	externalTeamRepository ExternalTeamRepository
	externalAPIClient      ExternalAPIClient
	logger                 Logger
}

func NewBackfillAliasesService(
	aliasRepository AliasRepository,
	externalTeamRepository ExternalTeamRepository,
	externalAPIClient ExternalAPIClient,
	logger Logger,
) *BackfillAliasesService {
	return &BackfillAliasesService{
		aliasRepository:        aliasRepository,
		externalTeamRepository: externalTeamRepository,
		externalAPIClient:      externalAPIClient,
		logger:                 logger,
	}
}

func (s *BackfillAliasesService) Backfill(ctx context.Context, dates []time.Time) error {
	s.logger.Info().Times("dates", dates).Msg("starting aliases backfill")

	teams, err := s.getTeams(ctx, dates)
	if err != nil {
		return fmt.Errorf("failed to get external matches: %w", err)
	}

	teams = deduplicateByTeamID(teams)

	s.saveTeams(ctx, teams)

	return nil
}

func (s *BackfillAliasesService) getTeams(ctx context.Context, dates []time.Time) ([]models.ExternalAPITeam, error) {
	const numberOfWorkers = 3
	jobs := make(chan struct{}, numberOfWorkers)
	wg := sync.WaitGroup{}
	var mutex = &sync.Mutex{}

	teams := []models.ExternalAPITeam{}

	for i, date := range dates {
		wg.Add(1)
		jobs <- struct{}{}

		dateOnly := date.Format(time.DateOnly)
		s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg("iterating through dates")

		go func(ctx context.Context, date time.Time) {
			allTeams, err := s.externalAPIClient.GetTeams(ctx, date)
			<-jobs

			if err != nil {
				s.logger.Error().Err(err).Int("iteration", i).Str("date", dateOnly).Msg("failed to get teams")
				return
			}

			s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg(fmt.Sprintf("number of all teams: %d", len(allTeams)))

			filteredTeams := filterTeamsByIncludedLeagues(allTeams, s.getIncludedLeagues())

			mutex.Lock()

			teams = append(teams, filteredTeams...)

			mutex.Unlock()

			s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg(fmt.Sprintf("number of filtered teams: %d", len(filteredTeams)))

			defer wg.Done()
		}(ctx, date)
	}

	wg.Wait()

	return teams, nil
}

func (s *BackfillAliasesService) getIncludedLeagues() []models.League {
	return []models.League{
		// european cups
		{Name: "Champions League", CountryCode: "INT"},
		{Name: "Europa League", CountryCode: "INT"},
		{Name: "Conference League", CountryCode: "INT"},
		// national teams
		{Name: "World Cup Qualification UEFA", CountryCode: "INT"},
		{Name: "World Cup Qualification CONMEBOL", CountryCode: "INT"},
		{Name: "Copa America", CountryCode: "INT"},
		{Name: "World Cup", CountryCode: "INT"},
		{Name: "Africa Cup of Nations", CountryCode: "INT"},
		// top leagues + ukrainian league
		{Name: "Premier League", CountryCode: "UKR"},
		{Name: "Premier League", CountryCode: "ENG"},
		{Name: "LaLiga", CountryCode: "ESP"},
		{Name: "Serie A", CountryCode: "ITA"},
		{Name: "Bundesliga", CountryCode: "GER"},
		{Name: "Ligue 1", CountryCode: "FRA"},
		{Name: "Eredivisie", CountryCode: "NED"},
		{Name: "Belgian Pro League", CountryCode: "BEL"},
		{Name: "Liga Portugal", CountryCode: "POR"},
		// only intersected with euro cups: Champions/Europa/Conference League
		{Name: "Super Lig", CountryCode: "TUR"},
		{Name: "Premiership", CountryCode: "SCO"},
		{Name: "1. Liga", CountryCode: "CZE"},
		{Name: "Super League", CountryCode: "SUI"},
		{Name: "Bundesliga", CountryCode: "AUT"},
		{Name: "Superligaen", CountryCode: "DEN"},
		{Name: "Eliteserien", CountryCode: "NOR"},
		{Name: "Ligat Ha'al", CountryCode: "ISR"},
		{Name: "Super League", CountryCode: "GRE"},
		{Name: "Super Liga", CountryCode: "SRB"},
		{Name: "Ekstraklasa", CountryCode: "POL"},
		{Name: "HNL", CountryCode: "CRO"},
		{Name: "Superliga", CountryCode: "ROU"},
		{Name: "Allsvenskan", CountryCode: "SWE"},
		// second leagues
		{Name: "2. Bundesliga", CountryCode: "GER"},
		{Name: "Championship", CountryCode: "ENG"},
		{Name: "LaLiga2", CountryCode: "ESP"},
		{Name: "Ligue 2", CountryCode: "FRA"},
		{Name: "Serie B", CountryCode: "ITA"},
		// other
		{Name: "Cup", CountryCode: "UKR"},
		{Name: "Premier League Qualification", CountryCode: "UKR"},
	}
}

func (s *BackfillAliasesService) saveTeams(ctx context.Context, teams []models.ExternalAPITeam) {
	numberOfSaved, numberOfExisted := 0, 0
	for i := range teams {
		_, err := s.aliasRepository.Find(ctx, teams[i].Name)
		if err == nil {
			s.logger.Debug().
				Str("alias", teams[i].Name).
				Uint("external_id", teams[i].ID).
				Msg("alias already exists")
			numberOfExisted++
			continue
		}

		externalTeam, errExtTeam := s.externalTeamRepository.FindExternalTeam(ctx, uint(teams[i].ID))
		if errExtTeam != nil && !errors.As(errExtTeam, &models.ResourceNotFoundError{}) {
			s.logger.Error().
				Str("alias", teams[i].Name).
				Uint("external_id", teams[i].ID).
				Err(errExtTeam).
				Msg("failed to find external team")
			continue
		}

		if externalTeam != nil {
			if errSaveForTeam := s.aliasRepository.SaveForTeam(ctx, teams[i].Name, externalTeam.TeamID); errSaveForTeam != nil {
				s.logger.Error().
					Str("alias", teams[i].Name).
					Uint("external_id", teams[i].ID).
					Err(errSaveForTeam).
					Msg("failed to save alias for existing team")
				continue
			}
			numberOfSaved++
			continue
		}

		errTrx := s.aliasRepository.SaveInTrx(ctx, teams[i].Name, uint(teams[i].ID))
		if errTrx != nil {
			s.logger.Error().
				Str("alias", teams[i].Name).
				Uint("external_id", teams[i].ID).
				Err(errTrx).
				Msg("failed to save alias")
			continue
		}
		numberOfSaved++
	}

	s.logger.Info().
		Int("number_of_saved", numberOfSaved).
		Int("number_of_existed", numberOfExisted).
		Msg("teams saving finished")
}

func deduplicateByTeamID(teams []models.ExternalAPITeam) []models.ExternalAPITeam {
	if len(teams) == 0 {
		return teams
	}

	uniqueByID := make(map[uint]models.ExternalAPITeam, len(teams))
	for _, team := range teams {
		uniqueByID[team.ID] = team
	}

	result := make([]models.ExternalAPITeam, 0, len(uniqueByID))
	for _, team := range uniqueByID {
		result = append(result, team)
	}

	return result
}

func filterTeamsByIncludedLeagues(allTeams []models.ExternalAPITeam, includedLeagues []models.League) []models.ExternalAPITeam {
	filtered := make([]models.ExternalAPITeam, 0, len(allTeams))
	for i := range allTeams {
		if teamBelongsToIncludedLeague(allTeams[i], includedLeagues) {
			filtered = append(filtered, allTeams[i])
		}
	}
	return filtered
}

func teamBelongsToIncludedLeague(team models.ExternalAPITeam, includedLeagues []models.League) bool {
	for i := range includedLeagues {
		if isLeagueTeam(includedLeagues[i], team) {
			return true
		}
	}
	return false
}

func isLeagueTeam(league models.League, team models.ExternalAPITeam) bool {
	return team.CountryCode == league.CountryCode && slices.Contains(team.LeagueNames, league.Name)
}

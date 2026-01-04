package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type BackfillAliasesService struct {
	aliasRepository AliasRepository
	fotmobClient    FotmobClient
	logger          Logger
}

func NewBackfillAliasesService(
	aliasRepository AliasRepository,
	fotmobClient FotmobClient,
	logger Logger,
) *BackfillAliasesService {
	return &BackfillAliasesService{
		aliasRepository: aliasRepository,
		fotmobClient:    fotmobClient,
		logger:          logger,
	}
}

func (s *BackfillAliasesService) Backfill(ctx context.Context, dates []time.Time) error {
	s.logger.Info().Times("dates", dates).Msg("starting aliases backfill")

	matches, err := s.getMatches(ctx, dates)
	if err != nil {
		return fmt.Errorf("failed to get external matches: %w", err)
	}

	teams := s.extractTeams(matches)
	s.saveTeams(ctx, teams)

	return nil
}

func (s *BackfillAliasesService) getMatches(ctx context.Context, dates []time.Time) (map[string][]ExternalAPIMatch, error) {
	const numberOfWorkers = 3
	jobs := make(chan struct{}, numberOfWorkers)
	wg := sync.WaitGroup{}
	var mutex = &sync.RWMutex{}

	matches := map[string][]ExternalAPIMatch{}

	for i, date := range dates {
		wg.Add(1)
		jobs <- struct{}{}

		dateOnly := date.Format(time.DateOnly)
		s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg("iterating through dates")

		go func(ctx context.Context, date time.Time) {
			result, err := s.fotmobClient.GetMatchesByDate(ctx, date)
			<-jobs

			if err != nil {
				s.logger.Error().Err(err).Int("iteration", i).Str("date", dateOnly).Msg("failed to get matches")
				return
			}

			s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg("successfully got matches")

			mutex.Lock()

			allLeagues, err := fromClientFotmobLeagues(*result)
			if err != nil {
				mutex.Unlock()
				s.logger.Error().Err(err).Int("iteration", i).Str("date", dateOnly).Msg("failed to map from client result")
				return
			}

			filteredLeagues := s.filterOutLeagues(allLeagues, s.getIncludedLeagues())
			for _, league := range filteredLeagues {
				matches[dateOnly] = append(matches[dateOnly], league.Matches...)
			}

			s.logger.Debug().Int("iteration", i).Str("date", dateOnly).Msg(fmt.Sprintf("found %d matches", len(matches[dateOnly])))

			mutex.Unlock()

			defer wg.Done()
		}(ctx, date)
	}

	wg.Wait()

	s.logger.Info().Int("number_of_matches", len(matches)).Msg("matches from all dates received")

	return matches, nil
}

func (s *BackfillAliasesService) filterOutLeagues(allLeagues []ExternalAPILeague, includedLeagues []ExternalAPILeague) []ExternalAPILeague {
	filtered := make([]ExternalAPILeague, 0, len(includedLeagues))
	for i := range allLeagues {
		if isIncludedLeague(allLeagues[i], includedLeagues) {
			filtered = append(filtered, allLeagues[i])
		}
	}

	return filtered
}

func (s *BackfillAliasesService) getIncludedLeagues() []ExternalAPILeague {
	return []ExternalAPILeague{
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
		{Name: "Liga Portugal", CountryCode: "POR"},
		{Name: "Belgian Pro League", CountryCode: "BEL"},
		// only intersected with euro cups: Champions/Europa/Conference League
		//{Name: "1. Lig", CountryCode: "TUR"},
		//{Name: "Premiership", CountryCode: "SCO"},
		//{Name: "1. Liga", CountryCode: "CZE"},
		//{Name: "Super League", CountryCode: "SUI"},
		//{Name: "Bundesliga", CountryCode: "AUT"},
		//{Name: "Superligaen", CountryCode: "DEN"},
		//{Name: "Eliteserien", CountryCode: "NOR"},
		//{Name: "Ligat Ha'al", CountryCode: "ISR"},
		//{Name: "Super League", CountryCode: "GRE"},
		//{Name: "Super Liga", CountryCode: "SRB"},
		//{Name: "Ekstraklasa", CountryCode: "POL"},
		//{Name: "HNL", CountryCode: "CRO"},
	}
}

func (s *BackfillAliasesService) extractTeams(matchesByDate map[string][]ExternalAPIMatch) []ExternalAPITeam {
	var teams []ExternalAPITeam
	for _, date := range matchesByDate {
		for _, match := range date {
			teams = append(teams, match.Home)
			teams = append(teams, match.Away)
		}
	}

	return teams
}

func (s *BackfillAliasesService) saveTeams(ctx context.Context, teams []ExternalAPITeam) {
	numberOfSaved, numberOfExisted := 0, 0
	for i := range teams {
		_, err := s.aliasRepository.Find(ctx, teams[i].Name)
		if err == nil {
			s.logger.Debug().
				Str("alias", teams[i].Name).
				Int("external_id", teams[i].ID).
				Msg("alias already exists")
			numberOfExisted++
			continue
		}

		errTrx := s.aliasRepository.SaveInTrx(ctx, teams[i].Name, uint(teams[i].ID))
		if errTrx != nil {
			s.logger.Error().
				Str("alias", teams[i].Name).
				Int("external_id", teams[i].ID).
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

func isIncludedLeague(league ExternalAPILeague, includedLeagues []ExternalAPILeague) bool {
	for i := range includedLeagues {
		if (includedLeagues[i].Name == league.Name || includedLeagues[i].ParentLeagueName == league.Name) &&
			includedLeagues[i].CountryCode == league.CountryCode {
			return true
		}
	}

	return false
}

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
		{Name: "Champions League", CountryCode: "INT"},  // 2025-12-09,2025-12-10
		{Name: "Europa League", CountryCode: "INT"},     // 2025-12-11
		{Name: "Conference League", CountryCode: "INT"}, // 2025-12-11
		// national teams
		{Name: "World Cup Qualification UEFA", CountryCode: "INT"},     // 2025-11-18,2025-11-17,2025-11-16,2025-11-15,2025-11-14,2025-11-13
		{Name: "World Cup Qualification CONMEBOL", CountryCode: "INT"}, // 2025-09-09,
		{Name: "Copa America", CountryCode: "INT"},                     // 2024-06-21,2024-06-22,2024-06-23,2024-06-24,2024-06-25,2024-06-26
		{Name: "World Cup", CountryCode: "INT"},                        // 2022-11-20,2022-11-21,2022-11-22,2022-11-23,2022-11-24
		{Name: "Africa Cup of Nations", CountryCode: "INT"},            // 2025-12-21,2025-12-22,2025-12-23,2025-12-24
		// top leagues + ukrainian league
		{Name: "Premier League", CountryCode: "UKR"},     // 2025-12-12,2025-12-13,2025-12-14
		{Name: "Premier League", CountryCode: "ENG"},     // 2026-01-06,2026-01-07,2026-01-08
		{Name: "LaLiga", CountryCode: "ESP"},             // 2025-12-19,2025-12-20,2025-12-21,2025-12-22
		{Name: "Serie A", CountryCode: "ITA"},            // 2026-01-10,2026-01-11,2026-01-12
		{Name: "Bundesliga", CountryCode: "GER"},         // 2025-12-19,2025-12-20,2025-12-21
		{Name: "Ligue 1", CountryCode: "FRA"},            // 2026-01-02,2026-01-03,2026-01-04
		{Name: "Eredivisie", CountryCode: "NED"},         // 2026-01-09,2026-01-10,2026-01-11
		{Name: "Belgian Pro League", CountryCode: "BEL"}, // 2025-12-19,2025-12-20,2025-12-21
		{Name: "Liga Portugal", CountryCode: "POR"},      // 2026-01-16,2026-01-17,2026-01-18,2026-01-19
		// only intersected with euro cups: Champions/Europa/Conference League
		{Name: "Super Lig", CountryCode: "TUR"},    // 2025-12-19,2025-12-20,2025-12-21,2025-12-22
		{Name: "Premiership", CountryCode: "SCO"},  // 2026-01-10,2026-01-11
		{Name: "1. Liga", CountryCode: "CZE"},      // 2025-12-13,2025-12-14
		{Name: "Super League", CountryCode: "SUI"}, // 2025-12-13,2025-12-14
		{Name: "Bundesliga", CountryCode: "AUT"},   // 2025-12-13,2025-12-14
		{Name: "Superligaen", CountryCode: "DEN"},  // 2025-12-05,2025-12-07,2025-12-08
		{Name: "Eliteserien", CountryCode: "NOR"},  // 2025-11-30
		{Name: "Ligat Ha'al", CountryCode: "ISR"},  // 2026-01-09,2026-01-10,2026-01-11
		{Name: "Super League", CountryCode: "GRE"}, // 2026-01-10,2026-01-11
		{Name: "Super Liga", CountryCode: "SRB"},   // 2025-12-20,2025-12-21,2025-12-22
		{Name: "Ekstraklasa", CountryCode: "POL"},  // 2025-12-05,2025-12-06,2025-12-07,2025-12-08
		{Name: "HNL", CountryCode: "CRO"},          // 2025-12-19,2025-12-20,2025-12-21
		{Name: "Superliga", CountryCode: "ROU"},    // 2025-12-19,2025-12-20,2025-12-21,2025-12-22
		{Name: "Allsvenskan", CountryCode: "SWE"},  // 2025-11-09
		// second leagues
		{Name: "2. Bundesliga", CountryCode: "GER"}, // 2026-01-16,2026-01-17,2026-01-18
		{Name: "Championship", CountryCode: "ENG"},  // 2026-01-16,2026-01-17
		{Name: "LaLiga2", CountryCode: "ESP"},       // 2026-01-16,2026-01-17,2026-01-18,2026-01-19
		{Name: "Ligue 2", CountryCode: "FRA"},       // 2026-01-16,2026-01-17,2026-01-18,2026-01-19
		{Name: "Serie B", CountryCode: "ITA"},       // 2026-01-16,2026-01-17,2026-01-18
		// other
		{Name: "Cup", CountryCode: "UKR"},                          // 2026-03-03
		{Name: "Premier League Qualification", CountryCode: "UKR"}, // 2025-05-29,2025-06-01

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
		if (includedLeagues[i].Name == league.Name || includedLeagues[i].Name == league.ParentLeagueName) &&
			includedLeagues[i].CountryCode == league.CountryCode {
			return true
		}
	}

	return false
}

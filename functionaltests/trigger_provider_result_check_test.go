//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
)

// TODO: port the remaining cases from trigger_result_check_test.go here (todo.md Feature #1).
func (s *FunctionalTestSuite) TestTriggerProviderResultCheck_MatchFinished() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	providerMatchID := uint(gofakeit.Uint32())
	providerMatch := testutils.CreateProviderMatch(s.T(), s.db, testutils.FakeRepositoryProviderMatch(func(pm *repository.ProviderMatch) {
		pm.MatchID = match.ID
		pm.ProviderMatchID = fmt.Sprintf("%d", providerMatchID)
		pm.StartsAt = match.StartsAt
		pm.Status = string(models.StatusMatchNotStarted)
	}))

	_ = testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
	}))

	checkResultTask := testutils.CreateCheckResultTask(s.T(), s.db, repository.CheckResultTask{
		MatchID:         match.ID,
		ProviderMatchID: &providerMatch.ID,
		ExecuteAt:       match.StartsAt.Add(115 * time.Minute),
	})

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].ID = providerMatchID
	matchesResponse.Leagues[0].Matches[0].Home.ID = int(teamSeeds[0].ProviderTeamID)
	matchesResponse.Leagues[0].Matches[0].Away.ID = int(teamSeeds[1].ProviderTeamID)
	matchesResponse.Leagues[0].Matches[0].StatusID = 6
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = match.StartsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string][]string{"date": {match.StartsAt.Format(fotmob.DateFormat)}, "timezone": {"UTC"}}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches",
		testutils.WithRequestHeaders(http.Header{"User-Agent": {"golang-app"}}),
		testutils.WithQueryParams(queryParams),
		testutils.WithResponseBody(string(jsonResponse)),
	)

	requestPayload := handler.TriggerProviderResultCheckRequest{ProviderMatchID: providerMatch.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/provider_result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Received),
			HomeScore:    &matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore:    &matchesResponse.Leagues[0].Matches[0].Away.Score,
			CreatedAt:    match.CreatedAt,
			UpdatedAt:    matches[0].UpdatedAt,
		},
	}, matches)

	checkResultTasks := testutils.ListCheckResultTasks(s.T(), s.db)
	s.Equal([]repository.CheckResultTask{
		{
			ID:              checkResultTask.ID,
			MatchID:         checkResultTask.MatchID,
			ProviderMatchID: checkResultTask.ProviderMatchID,
			Name:            checkResultTask.Name,
			AttemptNumber:   checkResultTask.AttemptNumber,
			ExecuteAt:       checkResultTask.ExecuteAt,
			CreatedAt:       checkResultTask.CreatedAt,
			UpdatedAt:       checkResultTasks[0].UpdatedAt,
		},
	}, checkResultTasks)

	providerMatches := testutils.ListProviderMatches(s.T(), s.db)
	providerStatus := "6"
	s.Equal([]repository.ProviderMatch{
		{
			ID:              providerMatch.ID,
			MatchID:         match.ID,
			Provider:        "fotmob",
			ProviderMatchID: fmt.Sprintf("%d", providerMatchID),
			StartsAt:        match.StartsAt.Truncate(time.Second),
			HomeScore:       matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore:       matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:          string(models.StatusMatchFinished),
			ProviderStatus:  &providerStatus,
			CreatedAt:       providerMatch.CreatedAt,
			UpdatedAt:       providerMatches[0].UpdatedAt,
		},
	}, providerMatches)
}

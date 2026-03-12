//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
)

func (s *FunctionalTestSuite) TestCreateMatch_Success() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	teamIDs := make([]uint, 0, len(aliases))
	externalTeamIDs := make([]uint, 0, len(teamIDs))
	for i := range aliases {
		teamID, externalTeam := testutils.SetupTeamWithRelations(s.T(), s.db, aliases[i], i+1)

		teamIDs = append(teamIDs, teamID)
		externalTeamIDs = append(externalTeamIDs, externalTeam.ID)
	}

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].Home.ID = externalTeamIDs[0]
	matchesResponse.Leagues[0].Matches[0].Away.ID = externalTeamIDs[1]
	matchesResponse.Leagues[0].Matches[0].StatusID = 1
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = startsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse))

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: "Arsenal",
		AliasAway: "Barcelona",
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/matches"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		MatchID uint `json:"match_id"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(uint(1), response.MatchID)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           1,
			HomeTeamID:   uint(teamIDs[0]),
			AwayTeamID:   uint(teamIDs[1]),
			StartsAt:     startsAt,
			ResultStatus: string(models.Scheduled),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        matchesResponse.Leagues[0].Matches[0].ID,
			MatchID:   response.MatchID,
			HomeScore: matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore: matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:    string(models.StatusMatchNotStarted),
		},
	}, externalMatches)

	checkResultTasks := testutils.ListCheckResultTasks(s.T(), s.db)
	s.Equal([]repository.CheckResultTask{
		{
			ID:            1,
			MatchID:       response.MatchID,
			Name:          fmt.Sprintf("projects/test-project/locations/europe-west3/queues/%s/tasks/match-%d-attempt-%d", "check-result", response.MatchID, 1),
			AttemptNumber: 1,
			ExecuteAt:     startsAt.Add(115 * time.Minute),
			CreatedAt:     checkResultTasks[0].CreatedAt,
		},
	}, checkResultTasks)
}

func (s *FunctionalTestSuite) TestCreateMatch_AlreadyExistsScheduled() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	teamIDs := make([]uint, 0, len(aliases))
	for i := range aliases {
		teamID, _ := testutils.SetupTeamWithRelations(s.T(), s.db, aliases[i], i+1)
		teamIDs = append(teamIDs, teamID)
	}

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamIDs[0]),
		AwayTeamID:   uint(teamIDs[1]),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	s.Require().NoError(err)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: "Arsenal",
		AliasAway: "Barcelona",
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/matches"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		MatchID uint `json:"match_id"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(created.ID, response.MatchID)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(1, len(matches))
	s.Equal(response.MatchID, matches[0].ID)
}

func (s *FunctionalTestSuite) TestCreateMatch_MatchNotFoundInExternalAPI() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	teamIDs := make([]uint, 0, len(aliases))
	for i := range aliases {
		teamID, _ := testutils.SetupTeamWithRelations(s.T(), s.db, aliases[i], i+1)
		teamIDs = append(teamIDs, teamID)
	}

	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, `{"leagues": [{"matches": []}]}`)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: "Arsenal",
		AliasAway: "Barcelona",
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/matches"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "match not found")
	s.Equal("unprocessable_content", response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

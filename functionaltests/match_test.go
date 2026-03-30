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
)

func (s *FunctionalTestSuite) TestCreateMatch_Success() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	s.Require().NoError(err)

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 1
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = startsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": startsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusOK, resp.StatusCode)

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
			HomeTeamID:   teamSeeds[0].TeamID,
			AwayTeamID:   teamSeeds[1].TeamID,
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

func (s *FunctionalTestSuite) TestCreateMatch_InvalidPayload() {
	requestPayload := handler.CreateMatchRequest{}

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

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "required")
	s.Equal(string(models.CodeInvalidRequest), response.Code)
}

func (s *FunctionalTestSuite) TestCreateMatch_AliasNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: "WrongAlias",
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "failed to find team alias")
	s.Equal(string(models.CodeResourceNotFound), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

func (s *FunctionalTestSuite) TestCreateMatch_AlreadyExistsScheduled() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   teamSeeds[0].TeamID,
		AwayTeamID:   teamSeeds[1].TeamID,
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	s.Require().NoError(err)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusOK, resp.StatusCode)

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

func (s *FunctionalTestSuite) TestCreateMatch_AlreadyExistsNonScheduled() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   teamSeeds[0].TeamID,
		AwayTeamID:   teamSeeds[1].TeamID,
		ResultStatus: string(models.Received),
	}
	_ = testutils.CreateMatch(s.T(), s.db, match)

	s.Require().NoError(err)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "match already exists with result status: received")
	s.Equal(string(models.CodeUnprocessableContent), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(1, len(matches))
}

func (s *FunctionalTestSuite) TestCreateMatch_ExternalAPIReturnsError() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-01T20:00:00Z")
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": startsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusInternalServerError, `internal server error`, queryParams)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("failed to get matches from external api: failed to fetch matches by date: failed to get matches by date, status code 500", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)
}

func (s *FunctionalTestSuite) TestCreateMatch_ExternalAPIReturnsInvalidResponseBody() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-01T20:00:00Z")
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": startsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, `!@#!@#`, queryParams)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("failed to get matches from external api: failed to fetch matches by date: failed to decode get matches by date response body: invalid character '!' looking for beginning of value", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)
}

func (s *FunctionalTestSuite) TestCreateMatch_MatchNotFoundInExternalAPI() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-03T20:00:00Z")
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": startsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, `{"leagues": [{"matches": []}]}`, queryParams)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "match not found")
	s.Equal(string(models.CodeUnprocessableContent), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

func (s *FunctionalTestSuite) TestCreateMatch_ExternalAPIStatusDoesntAllowScheduling() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-02T20:00:00Z")
	s.Require().NoError(err)

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 6
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = startsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": startsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: teamSeeds[0].Alias,
		AliasAway: teamSeeds[1].Alias,
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

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "result check scheduling is not allowed for this match, external match status is finished")
	s.Equal(string(models.CodeUnprocessableContent), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

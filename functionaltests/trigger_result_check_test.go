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

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: created.ID + 1}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("failed to get match by id: match not found: record not found", response.Error)
	s.Equal(string(models.CodeResourceNotFound), response.Code)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotScheduled() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Cancelled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: created.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_ExternalMatchRelationNotfound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: created.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("match relation external match does not exist", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_ExternalAPIReturnsError() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)
	_ = testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	queryParams := map[string]interface{}{"date": matchToCreate.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusInternalServerError, `internal server error`, queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("failed to get matches from external api: failed to fetch matches by date: failed to get matches by date, status code 500", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.APIError),
		},
	}, matches)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotFoundInExternalAPI() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)
	externalMatch := testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	queryParams := map[string]interface{}{"date": matchToCreate.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, `{"leagues": [{"matches": []}]}`, queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Cancelled),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        externalMatch.ID,
			MatchID:   match.ID,
			HomeScore: externalMatch.HomeScore,
			AwayScore: externalMatch.AwayScore,
			Status:    string(models.StatusMatchUnknown),
		},
	}, externalMatches)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchFoundWithUnexpectedStatus() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)
	externalMatch := testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].ID = externalMatch.ID
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 11111
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = match.StartsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": match.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Cancelled),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        externalMatch.ID,
			MatchID:   match.ID,
			HomeScore: matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore: matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:    string(models.StatusMatchUnknown),
		},
	}, externalMatches)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchFinished() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	externalMatch := testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	_ = testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
	}))

	checkResultTask := testutils.CreateCheckResultTask(s.T(), s.db, repository.CheckResultTask{MatchID: match.ID, ExecuteAt: match.StartsAt.Add(115 * time.Minute)})

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].ID = externalMatch.ID
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 6
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = match.StartsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": match.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Received),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        externalMatch.ID,
			MatchID:   match.ID,
			HomeScore: matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore: matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:    string(models.StatusMatchFinished),
		},
	}, externalMatches)

	checkResultTasks := testutils.ListCheckResultTasks(s.T(), s.db)
	s.Equal([]repository.CheckResultTask{
		{
			ID:            checkResultTask.ID,
			MatchID:       checkResultTask.MatchID,
			Name:          checkResultTask.Name,
			AttemptNumber: checkResultTask.AttemptNumber,
			ExecuteAt:     checkResultTask.ExecuteAt,
			CreatedAt:     checkResultTask.CreatedAt,
		},
	}, checkResultTasks)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotFinished() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	externalMatch := testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	_ = testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
	}))

	checkResultTask := testutils.CreateCheckResultTask(s.T(), s.db, repository.CheckResultTask{MatchID: match.ID, ExecuteAt: match.StartsAt.Add(115 * time.Minute)})

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].ID = externalMatch.ID
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 3 // live 2nd half
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = match.StartsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": match.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Scheduled),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        externalMatch.ID,
			MatchID:   match.ID,
			HomeScore: matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore: matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:    string(models.StatusMatchInProgress),
		},
	}, externalMatches)

	checkResultTasks := testutils.ListCheckResultTasks(s.T(), s.db)
	s.Equal([]repository.CheckResultTask{
		{
			ID:            checkResultTask.ID,
			MatchID:       match.ID,
			Name:          fmt.Sprintf("projects/test-project/locations/europe-west3/queues/check-result/tasks/match-%d-attempt-2", match.ID),
			AttemptNumber: checkResultTask.AttemptNumber + 1,
			ExecuteAt:     match.StartsAt.Add(115 * time.Minute).Add(5 * time.Minute),
			CreatedAt:     checkResultTask.CreatedAt,
		},
	}, checkResultTasks)
}

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotFinishedAndCheckResultTaskNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	externalMatch := testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = match.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

	_ = testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
	}))

	matchesResponse := testutils.FakeMatchesResponse()
	matchesResponse.Leagues[0].Matches[0].ID = externalMatch.ID
	matchesResponse.Leagues[0].Matches[0].Home.ID = teamSeeds[0].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].Away.ID = teamSeeds[1].ExternalTeamID
	matchesResponse.Leagues[0].Matches[0].StatusID = 3 // live 2nd half
	matchesResponse.Leagues[0].Matches[0].Status.UTCTime = match.StartsAt.UTC().Format(time.RFC3339)
	jsonResponse, err := json.Marshal(matchesResponse)
	s.Require().NoError(err)

	queryParams := map[string]interface{}{"date": match.StartsAt.Format(fotmob.DateFormat), "timezone": "Europe/London"}
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusOK, string(jsonResponse), queryParams)

	requestPayload := handler.TriggerResultCheckRequest{MatchID: match.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/result_check"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("match relation result check task doesn't exist", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal([]repository.Match{
		{
			ID:           match.ID,
			HomeTeamID:   match.HomeTeamID,
			AwayTeamID:   match.AwayTeamID,
			StartsAt:     match.StartsAt,
			ResultStatus: string(models.Scheduled),
		},
	}, matches)

	externalMatches := testutils.ListExternalMatches(s.T(), s.db)
	s.Equal([]repository.ExternalMatch{
		{
			ID:        externalMatch.ID,
			MatchID:   match.ID,
			HomeScore: matchesResponse.Leagues[0].Matches[0].Home.Score,
			AwayScore: matchesResponse.Leagues[0].Matches[0].Away.Score,
			Status:    string(models.StatusMatchInProgress),
		},
	}, externalMatches)
}

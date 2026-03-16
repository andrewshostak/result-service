//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
)

func (s *FunctionalTestSuite) TestTriggerResultCheck_MatchNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
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
		StartsAt:     gofakeit.Date(),
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
		StartsAt:     gofakeit.Date(),
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
	testutils.MockHTTPRequest(s.T(), s.smockerAdminURL, "/api/data/matches", http.MethodGet, http.StatusInternalServerError, `internal server error`)

	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)
	_ = testutils.CreateExternalMatch(s.T(), s.db, testutils.FakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = created.ID
		m.Status = string(models.StatusMatchNotStarted)
	}))

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
	s.Equal("failed to get matches from external api: failed to fetch matches by date: failed to get matches by date, status code 500", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)
}

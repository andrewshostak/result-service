//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
)

func (s *FunctionalTestSuite) TestCreateSubscription_Success() {
	now := time.Now()
	teamIDs := testutils.CreateTeams(s.T(), s.db)

	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	for i, teamID := range teamIDs {
		testutils.CreateAlias(s.T(), s.db, aliases[i], teamID)
	}

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamIDs[0]),
		AwayTeamID:   uint(teamIDs[1]),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.CreateSubscriptionRequest{
		MatchID:   created.ID,
		URL:       gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, created.ID)
	s.Equal(1, len(subs))
	s.Greater(subs[0].ID, uint(0))
	s.Equal(subs[0].MatchID, subs[0].MatchID)
	s.Equal(subs[0].Url, requestPayload.URL)
	s.Equal(subs[0].Key, requestPayload.SecretKey)
	s.Equal(subs[0].Status, string(models.PendingSub))
	s.GreaterOrEqual(subs[0].CreatedAt.Unix(), now.Unix())
	s.Nil(subs[0].NotifiedAt)
	s.Nil(subs[0].SubscriberError)
}

func (s *FunctionalTestSuite) TestCreateSubscription_InvalidPayload() {
	requestPayload := handler.CreateSubscriptionRequest{}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

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
	s.Contains(response.Error, "required")
	s.Equal("invalid_request", response.Code)
}

func (s *FunctionalTestSuite) TestCreateSubscription_MatchNotFound() {
	requestPayload := handler.CreateSubscriptionRequest{
		MatchID:   1,
		URL:       gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

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
	s.Equal("failed to get a match: match not found: record not found", response.Error)
	s.Equal("resource_not_found", response.Code)
}

func (s *FunctionalTestSuite) TestCreateSubscription_MatchResultNotScheduled() {
	teamIDs := testutils.CreateTeams(s.T(), s.db)

	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	for i, teamID := range teamIDs {
		testutils.CreateAlias(s.T(), s.db, aliases[i], teamID)
	}

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamIDs[0]),
		AwayTeamID:   uint(teamIDs[1]),
		ResultStatus: string(models.NotScheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.CreateSubscriptionRequest{
		MatchID:   created.ID,
		URL:       gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/subscriptions"
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
	s.Equal("match result status doesn't allow to create a subscription", response.Error)
	s.Equal("unprocessable_content", response.Code)
}

func (s *FunctionalTestSuite) TestCreateSubscription_SubscriptionAlreadyExists() {
	teamIDs := testutils.CreateTeams(s.T(), s.db)

	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	for i, teamID := range teamIDs {
		testutils.CreateAlias(s.T(), s.db, aliases[i], teamID)
	}

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamIDs[0]),
		AwayTeamID:   uint(teamIDs[1]),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	requestPayload := handler.CreateSubscriptionRequest{
		MatchID:   created.ID,
		URL:       gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	existingSub := testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: requestPayload.MatchID,
		Url:     requestPayload.URL,
		Key:     requestPayload.SecretKey,
	})

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, created.ID)
	s.Equal(1, len(subs))
	s.Equal(existingSub.ID, subs[0].ID)
}

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
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
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

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

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

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "required")
	s.Equal(string(models.CodeInvalidRequest), response.Code)
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

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("failed to get a match: match not found: record not found", response.Error)
	s.Equal(string(models.CodeResourceNotFound), response.Code)
}

func (s *FunctionalTestSuite) TestCreateSubscription_MatchResultNotScheduled() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
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

	s.Require().Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal("match result status doesn't allow to create a subscription", response.Error)
	s.Equal(string(models.CodeUnprocessableContent), response.Code)
}

func (s *FunctionalTestSuite) TestCreateSubscription_SubscriptionAlreadyExists() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	match := repository.Match{
		StartsAt:     gofakeit.Date(),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
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
		Status:  string(models.PendingSub),
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

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, created.ID)
	s.Equal(1, len(subs))
	s.Equal(existingSub.ID, subs[0].ID)
}

func (s *FunctionalTestSuite) TestDeleteSubscription_Success() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	subscriptionUrl := gofakeit.URL()

	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	createdMatch := testutils.CreateMatch(s.T(), s.db, match)

	_ = testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: createdMatch.ID,
		Url:     subscriptionUrl,
		Key:     secretKey,
		Status:  string(models.PendingSub),
	})

	_ = testutils.CreateCheckResultTask(s.T(), s.db, repository.CheckResultTask{MatchID: createdMatch.ID, Name: "hello/task/1", ExecuteAt: gofakeit.Date()})

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", subscriptionUrl)
	q.Add("secret_key", secretKey)
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, createdMatch.ID)
	s.Equal(0, len(subs))

	tasks := testutils.ListCheckResultTasks(s.T(), s.db)
	s.Equal(0, len(tasks))

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

func (s *FunctionalTestSuite) TestDeleteSubscription_InvalidPayload() {
	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
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

func (s *FunctionalTestSuite) TestDeleteSubscription_SuccessOtherMatchSubscriptionsExist() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	subscriptionUrl := gofakeit.URL()

	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	createdMatch := testutils.CreateMatch(s.T(), s.db, match)

	_ = testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: createdMatch.ID,
		Url:     subscriptionUrl,
		Key:     secretKey,
		Status:  string(models.PendingSub),
	})

	otherSubscription := testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: createdMatch.ID,
		Url:     gofakeit.URL(),
		Key:     gofakeit.Password(true, true, true, false, false, 10),
		Status:  string(models.PendingSub),
	})

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", subscriptionUrl)
	q.Add("secret_key", secretKey)
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, createdMatch.ID)
	s.Equal(1, len(subs))
	s.Equal(otherSubscription.ID, subs[0].ID)

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(1, len(matches))
}

func (s *FunctionalTestSuite) TestDeleteSubscription_CheckResultTaskRelationDoesNotExist() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	subscriptionUrl := gofakeit.URL()

	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Scheduled),
	}
	createdMatch := testutils.CreateMatch(s.T(), s.db, match)

	_ = testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: createdMatch.ID,
		Url:     subscriptionUrl,
		Key:     secretKey,
		Status:  string(models.PendingSub),
	})

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", subscriptionUrl)
	q.Add("secret_key", secretKey)
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	subs := testutils.ListSubscriptionsByMatch(s.T(), s.db, createdMatch.ID)
	s.Equal(0, len(subs))

	matches := testutils.ListMatches(s.T(), s.db)
	s.Equal(0, len(matches))
}

func (s *FunctionalTestSuite) TestDeleteSubscription_AliasNotFound() {
	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", "Alias1")
	q.Add("alias_away", "Alias2")
	q.Add("base_url", gofakeit.URL())
	q.Add("secret_key", gofakeit.Password(true, true, true, false, false, 10))
	req.URL.RawQuery = q.Encode()

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
	s.Contains(response.Error, "failed to find home team alias")
	s.Equal(string(models.CodeResourceNotFound), response.Code)
}

func (s *FunctionalTestSuite) TestDeleteSubscription_MatchNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", gofakeit.URL())
	q.Add("secret_key", gofakeit.Password(true, true, true, false, false, 10))
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
}

func (s *FunctionalTestSuite) TestDeleteSubscription_SubscriptionNotFound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")

	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.NotScheduled),
	}
	_ = testutils.CreateMatch(s.T(), s.db, match)

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", gofakeit.URL())
	q.Add("secret_key", gofakeit.Password(true, true, true, false, false, 10))
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
}

func (s *FunctionalTestSuite) TestDeleteSubscription_SubscriberAlreadyNotified() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	subscriptionUrl := gofakeit.URL()

	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Received),
	}
	createdMatch := testutils.CreateMatch(s.T(), s.db, match)

	_ = testutils.CreateSubscription(s.T(), s.db, repository.Subscription{
		MatchID: createdMatch.ID,
		Url:     subscriptionUrl,
		Key:     secretKey,
		Status:  string(models.SuccessfulSub),
	})

	url := s.apiBaseURL + "/v1/subscriptions"
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("starts_at", startsAt.Format(time.RFC3339))
	q.Add("alias_home", teamSeeds[0].Alias)
	q.Add("alias_away", teamSeeds[1].Alias)
	q.Add("base_url", subscriptionUrl)
	q.Add("secret_key", secretKey)
	req.URL.RawQuery = q.Encode()

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
	s.Contains(response.Error, "not allowed to delete successfully notified subscription")
	s.Equal(string(models.CodeUnprocessableContent), response.Code)
}

//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
)

func (s *FunctionalTestSuite) TestTriggerSubscriberNotification_SubscriptionNotFound() {
	subscriptionID := uint(gofakeit.Uint8())
	requestPayload := handler.TriggerSubscriptionNotificationRequest{SubscriptionID: subscriptionID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/subscriber_notification"
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
	s.Equal(fmt.Sprintf("failed to get subscription by id: subscription with id %d not found: record not found", subscriptionID), response.Error)
	s.Equal(string(models.CodeResourceNotFound), response.Code)
}

func (s *FunctionalTestSuite) TestTriggerSubscriberNotification_SubscriptionAlreadyNotified() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Received),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	subscription := testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
		sub.Status = string(models.SuccessfulSub)
	}))

	requestPayload := handler.TriggerSubscriptionNotificationRequest{SubscriptionID: subscription.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/subscriber_notification"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer anything")

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusNoContent, resp.StatusCode)

	subscriptions := testutils.ListSubscriptionsByMatch(s.T(), s.db, match.ID)
	s.Equal(subscription.ID, subscriptions[0].ID)
	s.Equal(match.ID, subscriptions[0].MatchID)
	s.Equal(subscription.Url, subscriptions[0].Url)
	s.Equal(subscription.Key, subscriptions[0].Key)
	s.Equal(subscription.CreatedAt, subscriptions[0].CreatedAt)
	s.Equal(subscription.Status, subscriptions[0].Status)
	s.Nil(subscriptions[0].SubscriberError)
	s.Nil(subscriptions[0].NotifiedAt)
}

func (s *FunctionalTestSuite) TestTriggerSubscriberNotification_ExternalMatchRelationNotfound() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	matchToCreate := repository.Match{
		StartsAt:     testutils.RandomFutureDate(s.T()),
		HomeTeamID:   uint(teamSeeds[0].TeamID),
		AwayTeamID:   uint(teamSeeds[1].TeamID),
		ResultStatus: string(models.Received),
	}
	match := testutils.CreateMatch(s.T(), s.db, matchToCreate)

	subscription := testutils.CreateSubscription(s.T(), s.db, testutils.FakeRepositorySubscription(func(sub *repository.Subscription) {
		sub.MatchID = match.ID
		sub.Url = gofakeit.URL()
		sub.Key = gofakeit.UUID()
		sub.Status = string(models.PendingSub)
	}))

	requestPayload := handler.TriggerSubscriptionNotificationRequest{SubscriptionID: subscription.ID}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/triggers/subscriber_notification"
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
	s.Equal("match relation external match doesn't exist", response.Error)
	s.Equal(string(models.CodeInternalServerError), response.Code)
}

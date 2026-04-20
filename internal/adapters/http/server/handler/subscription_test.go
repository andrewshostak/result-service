package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionHandler_Create(t *testing.T) {
	validRequestBody := handler.CreateSubscriptionRequest{
		MatchID:   uint(gofakeit.Uint16()),
		URL:       gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	validRequestBodyBytes, err := json.Marshal(validRequestBody)
	require.NoError(t, err)

	tests := []struct {
		name                 string
		request              *http.Request
		subscriptionService  func(t *testing.T) *mocks.SubscriptionService
		expectedStatus       int
		expectedResponseBody interface{}
	}{
		{
			name:    "it returns 400 when request body is invalid",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				return mocks.NewSubscriptionService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInvalidRequest),
				Error: "Key: 'CreateSubscriptionRequest.MatchID' Error:Field validation for 'MatchID' failed on the 'required' tag\nKey: 'CreateSubscriptionRequest.URL' Error:Field validation for 'URL' failed on the 'required' tag\nKey: 'CreateSubscriptionRequest.SecretKey' Error:Field validation for 'SecretKey' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 400 when subscription service returns a resource not found error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(models.NewResourceNotFoundError(errors.New("match not found"))).
					Once()
				return m
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeResourceNotFound),
				Error: "match not found",
			},
		},
		{
			name:    "it returns 422 when subscription service returns an unprocessable content error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(models.NewUnprocessableContentError(errors.New("match result status doesn't allow to create a subscription"))).
					Once()
				return m
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeUnprocessableContent),
				Error: "match result status doesn't allow to create a subscription",
			},
		},
		{
			name:    "it returns 500 when subscription service returns an unexpected error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(errors.New("unexpected error")).
					Once()
				return m
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInternalServerError),
				Error: "unexpected error",
			},
		},
		{
			name:    "success - it returns 204",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(nil).
					Once()
				return m
			},
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = tt.request

			h := handler.NewSubscriptionHandler(tt.subscriptionService(t))
			h.Create(c)
			c.Writer.Flush() //  Need this line because with gin.CreateTestContext in unit tests, the response writer doesn't automatically flush the headers, so the status code never gets written and defaults to 200

			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedResponseBody != nil {
				expectedBody, err := json.Marshal(tt.expectedResponseBody)
				require.NoError(t, err)
				assert.JSONEq(t, string(expectedBody), w.Body.String())
			}
		})
	}
}

func TestSubscriptionHandler_Delete(t *testing.T) {
	startsAt := gofakeit.Date().UTC().Truncate(time.Second)

	validRequestParams := handler.DeleteSubscriptionRequest{
		StartsAt:  startsAt,
		AliasHome: gofakeit.Word(),
		AliasAway: gofakeit.Word(),
		BaseURL:   gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	validRequestURL := func() string {
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		q := req.URL.Query()
		q.Add("starts_at", startsAt.Format(time.RFC3339))
		q.Add("alias_home", validRequestParams.AliasHome)
		q.Add("alias_away", validRequestParams.AliasAway)
		q.Add("base_url", validRequestParams.BaseURL)
		q.Add("secret_key", validRequestParams.SecretKey)
		req.URL.RawQuery = q.Encode()
		return req.URL.String()
	}()

	tests := []struct {
		name                 string
		request              *http.Request
		subscriptionService  func(t *testing.T) *mocks.SubscriptionService
		expectedStatus       int
		expectedResponseBody interface{}
	}{
		{
			name:    "it returns 400 when query params are missing",
			request: httptest.NewRequest(http.MethodDelete, "/", nil),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				return mocks.NewSubscriptionService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInvalidRequest),
				Error: "Key: 'DeleteSubscriptionRequest.StartsAt' Error:Field validation for 'StartsAt' failed on the 'required' tag\nKey: 'DeleteSubscriptionRequest.AliasHome' Error:Field validation for 'AliasHome' failed on the 'required' tag\nKey: 'DeleteSubscriptionRequest.AliasAway' Error:Field validation for 'AliasAway' failed on the 'required' tag\nKey: 'DeleteSubscriptionRequest.BaseURL' Error:Field validation for 'BaseURL' failed on the 'required' tag\nKey: 'DeleteSubscriptionRequest.SecretKey' Error:Field validation for 'SecretKey' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 400 when subscription service returns a resource not found error",
			request: httptest.NewRequest(http.MethodDelete, validRequestURL, nil),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Delete", mock.AnythingOfType("context.backgroundCtx"), validRequestParams.ToDomain()).
					Return(models.NewResourceNotFoundError(errors.New("subscription not found"))).
					Once()
				return m
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeResourceNotFound),
				Error: "subscription not found",
			},
		},
		{
			name:    "it returns 422 when subscription service returns an unprocessable content error",
			request: httptest.NewRequest(http.MethodDelete, validRequestURL, nil),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Delete", mock.AnythingOfType("context.backgroundCtx"), validRequestParams.ToDomain()).
					Return(models.NewUnprocessableContentError(errors.New("not allowed to delete successfully notified subscription"))).
					Once()
				return m
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeUnprocessableContent),
				Error: "not allowed to delete successfully notified subscription",
			},
		},
		{
			name:    "it returns 500 when subscription service returns an unexpected error",
			request: httptest.NewRequest(http.MethodDelete, validRequestURL, nil),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Delete", mock.AnythingOfType("context.backgroundCtx"), validRequestParams.ToDomain()).
					Return(errors.New("unexpected error")).
					Once()
				return m
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInternalServerError),
				Error: "unexpected error",
			},
		},
		{
			name:    "success - it returns 204",
			request: httptest.NewRequest(http.MethodDelete, validRequestURL, nil),
			subscriptionService: func(t *testing.T) *mocks.SubscriptionService {
				t.Helper()
				m := mocks.NewSubscriptionService(t)
				m.On("Delete", mock.AnythingOfType("context.backgroundCtx"), validRequestParams.ToDomain()).
					Return(nil).
					Once()
				return m
			},
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = tt.request

			h := handler.NewSubscriptionHandler(tt.subscriptionService(t))
			h.Delete(c)
			c.Writer.Flush()

			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedResponseBody != nil {
				expectedBody, err := json.Marshal(tt.expectedResponseBody)
				require.NoError(t, err)
				assert.JSONEq(t, string(expectedBody), w.Body.String())
			}
		})
	}
}

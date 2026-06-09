package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTriggerHandler_CheckResult(t *testing.T) {
	validRequestBody := handler.TriggerResultCheckRequest{MatchID: uint(gofakeit.Uint16())}

	validRequestBodyBytes, err := json.Marshal(validRequestBody)
	require.NoError(t, err)

	tests := []struct {
		name                      string
		request                   *http.Request
		resultCheckerService      func(t *testing.T) *mocks.ResultCheckerService
		subscriberNotifierService func(t *testing.T) *mocks.SubscriberNotifierService
		expectedStatus            int
		expectedResponseBody      interface{}
	}{
		{
			name:    "it returns 400 when request body is invalid",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)),
			resultCheckerService: func(t *testing.T) *mocks.ResultCheckerService {
				t.Helper()
				return mocks.NewResultCheckerService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInvalidRequest),
				Error: "Key: 'TriggerResultCheckRequest.MatchID' Error:Field validation for 'MatchID' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 400 when service returns a resource not found error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			resultCheckerService: func(t *testing.T) *mocks.ResultCheckerService {
				t.Helper()
				m := mocks.NewResultCheckerService(t)
				m.On("CheckResult", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.MatchID).
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
			name:    "it returns 500 when service returns an unexpected error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			resultCheckerService: func(t *testing.T) *mocks.ResultCheckerService {
				t.Helper()
				m := mocks.NewResultCheckerService(t)
				m.On("CheckResult", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.MatchID).
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
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			resultCheckerService: func(t *testing.T) *mocks.ResultCheckerService {
				t.Helper()
				m := mocks.NewResultCheckerService(t)
				m.On("CheckResult", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.MatchID).
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

			subscriberNotifier := mocks.NewSubscriberNotifierService(t)

			h := handler.NewTriggerHandler(tt.resultCheckerService(t), subscriberNotifier)
			h.CheckResult(c)
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

func TestTriggerHandler_NotifySubscriber(t *testing.T) {
	validRequestBody := handler.TriggerSubscriptionNotificationRequest{SubscriptionID: uint(gofakeit.Uint16())}

	validRequestBodyBytes, err := json.Marshal(validRequestBody)
	require.NoError(t, err)

	tests := []struct {
		name                      string
		request                   *http.Request
		resultCheckerService      func(t *testing.T) *mocks.ResultCheckerService
		subscriberNotifierService func(t *testing.T) *mocks.SubscriberNotifierService
		expectedStatus            int
		expectedResponseBody      interface{}
	}{
		{
			name:    "it returns 400 when request body is invalid",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)),
			subscriberNotifierService: func(t *testing.T) *mocks.SubscriberNotifierService {
				t.Helper()
				return mocks.NewSubscriberNotifierService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInvalidRequest),
				Error: "Key: 'TriggerSubscriptionNotificationRequest.SubscriptionID' Error:Field validation for 'SubscriptionID' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 400 when service returns a resource not found error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			subscriberNotifierService: func(t *testing.T) *mocks.SubscriberNotifierService {
				t.Helper()
				m := mocks.NewSubscriberNotifierService(t)
				m.On("NotifySubscriber", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.SubscriptionID).
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
			name:    "it returns 500 when service returns an unexpected error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			subscriberNotifierService: func(t *testing.T) *mocks.SubscriberNotifierService {
				t.Helper()
				m := mocks.NewSubscriberNotifierService(t)
				m.On("NotifySubscriber", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.SubscriptionID).
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
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(string(validRequestBodyBytes))),
			subscriberNotifierService: func(t *testing.T) *mocks.SubscriberNotifierService {
				t.Helper()
				m := mocks.NewSubscriberNotifierService(t)
				m.On("NotifySubscriber", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.SubscriptionID).
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

			resultCheckerService := mocks.NewResultCheckerService(t)

			h := handler.NewTriggerHandler(resultCheckerService, tt.subscriberNotifierService(t))
			h.NotifySubscriber(c)
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

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

func TestMatchHandler_Create(t *testing.T) {
	validRequestBody := handler.CreateMatchRequest{
		StartsAt:  time.Now().UTC().Truncate(time.Second),
		AliasHome: gofakeit.Word(),
		AliasAway: gofakeit.Word(),
	}

	validRequestBodyBytes, err := json.Marshal(validRequestBody)
	require.NoError(t, err)

	matchID := uint(gofakeit.Uint16())

	tests := []struct {
		name                 string
		request              *http.Request
		matchService         func(t *testing.T) *mocks.MatchService
		expectedStatus       int
		expectedResponseBody interface{}
	}{
		{
			name:    "it returns 400 when request body is invalid",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)),
			matchService: func(t *testing.T) *mocks.MatchService {
				t.Helper()
				return mocks.NewMatchService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: gin.H{
				"code":  string(models.CodeInvalidRequest),
				"error": "Key: 'CreateMatchRequest.StartsAt' Error:Field validation for 'StartsAt' failed on the 'required' tag\nKey: 'CreateMatchRequest.AliasHome' Error:Field validation for 'AliasHome' failed on the 'required' tag\nKey: 'CreateMatchRequest.AliasAway' Error:Field validation for 'AliasAway' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 422 when match service returns an unprocessable content error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			matchService: func(t *testing.T) *mocks.MatchService {
				t.Helper()
				m := mocks.NewMatchService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(uint(0), models.NewUnprocessableContentError(errors.New("match already exists"))).
					Once()
				return m
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeUnprocessableContent),
				Error: "match already exists",
			},
		},
		{
			name:    "it returns 400 when match service returns a resource not found error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			matchService: func(t *testing.T) *mocks.MatchService {
				t.Helper()
				m := mocks.NewMatchService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(uint(0), models.NewResourceNotFoundError(errors.New("alias not found"))).
					Once()
				return m
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeResourceNotFound),
				Error: "alias not found",
			},
		},
		{
			name:    "it returns 500 when match service returns an unexpected error",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			matchService: func(t *testing.T) *mocks.MatchService {
				t.Helper()
				m := mocks.NewMatchService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(uint(0), errors.New("unexpected error")).
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
			name:    "success - it returns 200 with match id",
			request: httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(validRequestBodyBytes)),
			matchService: func(t *testing.T) *mocks.MatchService {
				t.Helper()
				m := mocks.NewMatchService(t)
				m.On("Create", mock.AnythingOfType("context.backgroundCtx"), validRequestBody.ToDomain()).
					Return(matchID, nil).
					Once()
				return m
			},
			expectedStatus:       http.StatusOK,
			expectedResponseBody: gin.H{"match_id": float64(matchID)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = tt.request

			h := handler.NewMatchHandler(tt.matchService(t))
			h.Create(c)
			c.Writer.Flush()

			require.Equal(t, tt.expectedStatus, w.Code)

			expectedBody, err := json.Marshal(tt.expectedResponseBody)
			require.NoError(t, err)
			assert.JSONEq(t, string(expectedBody), w.Body.String())
		})
	}
}

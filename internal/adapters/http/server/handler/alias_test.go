package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasHandler_Search(t *testing.T) {
	validRequest := httptest.NewRequest(http.MethodGet, "/?search=hello", nil)

	tests := []struct {
		name                 string
		request              *http.Request
		aliasService         func(t *testing.T) *mocks.AliasService
		expectedStatus       int
		expectedResponseBody interface{}
	}{
		{
			name:    "success - it returns 200",
			request: validRequest,
			aliasService: func(t *testing.T) *mocks.AliasService {
				t.Helper()
				m := mocks.NewAliasService(t)
				m.On("Search", validRequest.Context(), "hello").Return([]string{"hello", "world"}, nil).Once()
				return m
			},
			expectedStatus:       http.StatusOK,
			expectedResponseBody: gin.H{"aliases": []string{"hello", "world"}},
		},
		{
			name:    "it returns 400 when search query is empty",
			request: httptest.NewRequest(http.MethodGet, "/", nil),
			aliasService: func(t *testing.T) *mocks.AliasService {
				t.Helper()
				return mocks.NewAliasService(t)
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInvalidRequest),
				Error: "Key: 'SearchAliasRequest.Search' Error:Field validation for 'Search' failed on the 'required' tag",
			},
		},
		{
			name:    "it returns 500 when alias service returns an error",
			request: validRequest,
			aliasService: func(t *testing.T) *mocks.AliasService {
				t.Helper()
				m := mocks.NewAliasService(t)
				m.On("Search", validRequest.Context(), "hello").Return(nil, errors.New("unknown error")).Once()
				return m
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponseBody: handler.ErrorResponse{
				Code:  string(models.CodeInternalServerError),
				Error: "unknown error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = tt.request

			h := handler.NewAliasHandler(tt.aliasService(t))
			h.Search(c)
			c.Writer.Flush()

			require.Equal(t, tt.expectedStatus, w.Code)

			expectedBody, err := json.Marshal(tt.expectedResponseBody)
			require.NoError(t, err)
			assert.JSONEq(t, string(expectedBody), w.Body.String())
		})
	}
}

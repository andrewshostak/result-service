package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecovery(t *testing.T) {
	tests := []struct {
		name          string
		panicValue    any
		expectedError string
	}{
		{
			name:          "it recovers from a panic with a string value",
			panicValue:    "boom",
			expectedError: "panic: boom",
		},
		{
			name:          "it recovers from a panic with an error value",
			panicValue:    errors.New("unexpected nil pointer"),
			expectedError: "panic: unexpected nil pointer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured *gin.Context

			r := gin.New()
			r.Use(middleware.Recovery())
			r.Use(func(c *gin.Context) {
				captured = c
				c.Next()
			})
			r.GET("/", func(c *gin.Context) {
				panic(tt.panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.Equal(t, []string{tt.expectedError}, captured.Errors.Errors())
		})
	}
}

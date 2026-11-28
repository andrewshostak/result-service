package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestErrorLogger(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		err          error
		expectLogged bool
	}{
		{
			name:         "it does not log when status is below 400",
			status:       http.StatusOK,
			expectLogged: false,
		},
		{
			name:         "it logs without an error field on a 408 timeout (handlers ran in a copied context, so no error is recorded)",
			status:       http.StatusRequestTimeout,
			expectLogged: true,
		},
		{
			name:         "it logs with an error field when status is 400 or above and an error was recorded",
			status:       http.StatusBadRequest,
			err:          errors.New("invalid request"),
			expectLogged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/path", nil)
			c.Status(tt.status)
			if tt.err != nil {
				_ = c.Error(tt.err)
			}

			middleware.ErrorLogger(&logger)(c)

			if !tt.expectLogged {
				require.Empty(t, buf.String())
				return
			}

			var logged map[string]interface{}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &logged))

			require.Equal(t, "request failed", logged["message"])
			require.Equal(t, float64(tt.status), logged["status"])
			require.Equal(t, http.MethodGet, logged["method"])
			require.Equal(t, "/path", logged["path"])

			if tt.err != nil {
				require.Equal(t, tt.err.Error(), logged["error"])
			} else {
				require.NotContains(t, logged, "error")
			}
		})
	}
}

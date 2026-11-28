package middleware_test

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAPIKeyAuth(t *testing.T) {
	secret := "test-secret"
	validAPIKey := "valid-api-key"
	hashedValidAPIKey := hashAPIKey(validAPIKey, secret)

	tests := []struct {
		name        string
		header      string
		expectAbort bool
	}{
		{
			name:        "it aborts with 401 when Authorization header is missing",
			header:      "",
			expectAbort: true,
		},
		{
			name:        "it aborts with 401 when api key is invalid",
			header:      "invalid-api-key",
			expectAbort: true,
		},
		{
			name:        "it does not abort when api key is valid",
			header:      validAPIKey,
			expectAbort: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("Authorization", tt.header)

			middleware.APIKeyAuth([]string{hashedValidAPIKey}, secret)(c)

			assert.Equal(t, tt.expectAbort, c.IsAborted())
			if tt.expectAbort {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		})
	}
}

func hashAPIKey(apiKey, secret string) string {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(apiKey))
	return hex.EncodeToString(h.Sum(nil))
}

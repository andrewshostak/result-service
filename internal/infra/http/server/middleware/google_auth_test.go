package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/idtoken"
)

func TestValidateGoogleAuth(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		validator   func(t *testing.T) *mocks.GoogleTokenValidator
		expectAbort bool
	}{
		{
			name:   "it aborts with 401 when Authorization header is missing",
			header: "",
			validator: func(t *testing.T) *mocks.GoogleTokenValidator {
				t.Helper()
				return mocks.NewGoogleTokenValidator(t)
			},
			expectAbort: true,
		},
		{
			name:   "it aborts with 401 when the header has no Bearer prefix",
			header: "some-token",
			validator: func(t *testing.T) *mocks.GoogleTokenValidator {
				t.Helper()
				return mocks.NewGoogleTokenValidator(t)
			},
			expectAbort: true,
		},
		{
			name:   "it aborts with 401 when the validator rejects the token",
			header: "Bearer some-token",
			validator: func(t *testing.T) *mocks.GoogleTokenValidator {
				t.Helper()
				m := mocks.NewGoogleTokenValidator(t)
				m.On("Validate", mock.AnythingOfType("context.backgroundCtx"), "some-token", "target-audience").
					Return(nil, errors.New("invalid token")).
					Once()
				return m
			},
			expectAbort: true,
		},
		{
			name:   "it does not abort when the validator accepts the token",
			header: "Bearer some-token",
			validator: func(t *testing.T) *mocks.GoogleTokenValidator {
				t.Helper()
				m := mocks.NewGoogleTokenValidator(t)
				m.On("Validate", mock.AnythingOfType("context.backgroundCtx"), "some-token", "target-audience").
					Return(&idtoken.Payload{}, nil).
					Once()
				return m
			},
			expectAbort: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("Authorization", tt.header)

			middleware.ValidateGoogleAuth(tt.validator(t), "target-audience")(c)

			assert.Equal(t, tt.expectAbort, c.IsAborted())
			if tt.expectAbort {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		})
	}
}

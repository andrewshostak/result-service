package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

type GoogleTokenValidator interface {
	Validate(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)
}

func ValidateGoogleAuth(validator GoogleTokenValidator, targetAudience string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorization)
		if authHeader == "" {
			abortWithError(c, http.StatusUnauthorized, errors.New("missing authorization header"))
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			abortWithError(c, http.StatusUnauthorized, errors.New("invalid token format"))
			return
		}

		if _, err := validator.Validate(c.Request.Context(), token, targetAudience); err != nil {
			abortWithError(c, http.StatusUnauthorized, fmt.Errorf("invalid google token: %w", err))
			return
		}

		c.Next()
	}
}

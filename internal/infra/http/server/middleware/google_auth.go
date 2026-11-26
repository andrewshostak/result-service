package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

func ValidateGoogleAuth(targetAudience string) gin.HandlerFunc {
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

		if gin.Mode() == gin.TestMode {
			c.Next()
			return
		}

		_, err := idtoken.Validate(c.Request.Context(), token, targetAudience)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, fmt.Errorf("invalid google token: %w", err))
			return
		}

		c.Next()
	}
}

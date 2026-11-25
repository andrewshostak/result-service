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
			err := errors.New("missing authorization header")
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			err := errors.New("invalid token format")
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		if gin.Mode() == gin.TestMode {
			c.Next()
			return
		}

		_, err := idtoken.Validate(c.Request.Context(), token, targetAudience)
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("invalid google token: %s", err.Error())})
			return
		}

		c.Next()
	}
}

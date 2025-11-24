package middleware

import (
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
			return
		}

		_, err := idtoken.Validate(c.Request.Context(), token, targetAudience)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("invalid google token: %s", err.Error())})
			return
		}

		c.Next()
	}
}

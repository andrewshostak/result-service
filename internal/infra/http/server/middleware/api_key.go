package middleware

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const authorization = "Authorization"

func APIKeyAuth(hashedAPIKeys []string, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(authorization)

		if !isValidAPIKey(apiKey, hashedAPIKeys, secret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}

		c.Next()
	}
}

func isValidAPIKey(apiKey string, hashedAPIKeys []string, secret string) bool {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(apiKey))
	sha := hex.EncodeToString(h.Sum(nil))

	for _, hashedAPIKey := range hashedAPIKeys {
		if sha == hashedAPIKey {
			return true
		}
	}

	return false
}

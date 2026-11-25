package middleware

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const authorization = "Authorization"

func APIKeyAuth(hashedAPIKeys []string, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(authorization)

		if !isValidAPIKey(apiKey, hashedAPIKeys, secret) {
			err := errors.New("invalid api key")
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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

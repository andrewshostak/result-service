package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func Timeout(t time.Duration) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(t),
		timeout.WithResponse(TimeoutResponse),
	)
}

func TimeoutResponse(c *gin.Context) {
	err := errors.New("timeout")
	_ = c.Error(err)
	c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Error(), "code": models.CodeTimeout})
}

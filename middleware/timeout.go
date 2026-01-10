package middleware

import (
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/errs"
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
	c.JSON(http.StatusRequestTimeout, gin.H{"error": "timeout", "code": errs.CodeTimeout})
}

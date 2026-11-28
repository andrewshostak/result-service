package middleware

import (
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

// TODO: gin-contrib/timeout runs the handler goroutine on the same *gin.Context that this
// middleware's own select uses to call c.AbortWithStatus() on timeout, with no synchronization
// between the two (confirmed via -race: concurrent writes to c.index from Abort() and Next()).
// It also returns a shared response buffer to its pool before the handler goroutine is guaranteed
// to be done writing to it. Rewrite using the standard http.TimeoutHandler wrapping the whole
// *gin.Engine (as done in our other Go services), where the handler goroutine owns its own
// *gin.Context entirely and the only state shared with the timeout select (timeoutWriter) is
// guarded by a real sync.Mutex. See https://github.com/gin-contrib/timeout/issues/45.
func Timeout(t time.Duration) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(t),
		timeout.WithResponse(TimeoutResponse),
	)
}

func TimeoutResponse(c *gin.Context) {
	c.JSON(http.StatusRequestTimeout, gin.H{"error": "timeout", "code": models.CodeTimeout})
}

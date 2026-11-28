package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func ErrorLogger(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if errors.Is(c.Request.Context().Err(), context.DeadlineExceeded) { // the timeout wrapper already answered the client; this status/error reflects a handler that kept running after that, not what the client received
			return
		}

		status := c.Writer.Status()
		if status < http.StatusBadRequest {
			return
		}

		event := logger.Error().Int("status", status).Str("method", c.Request.Method).Str("path", c.Request.URL.Path)
		if len(c.Errors) > 0 {
			event = event.Err(c.Errors[0].Err)
		}

		event.Msg("request failed")
	}
}

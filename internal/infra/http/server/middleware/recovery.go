package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		_ = c.Error(fmt.Errorf("panic: %v", err))
		c.Status(http.StatusInternalServerError)
	})
}

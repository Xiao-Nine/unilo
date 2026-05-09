package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", "panic", r, "path", c.Request.URL.Path)
				response.Error(c, apperror.New(500, "internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}

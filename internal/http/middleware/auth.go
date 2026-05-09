package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"unilo/internal/auth"
	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

func Auth(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, apperror.Unauthorized("authorization bearer token is required"))
			c.Abort()
			return
		}
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := tokens.VerifyAccess(tokenString)
		if err != nil {
			response.Error(c, apperror.Unauthorized("access token is invalid"))
			c.Abort()
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			response.Error(c, apperror.Unauthorized("access token is invalid"))
			c.Abort()
			return
		}
		c.Set(auth.ContextUserIDKey, userID)
		c.Next()
	}
}

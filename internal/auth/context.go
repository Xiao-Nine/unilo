package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ContextUserIDKey = "current_user_id"

func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(ContextUserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	return userID, ok
}

package auth

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_refresh_tokens_user_expires"`
	TokenHash string     `gorm:"size:255;uniqueIndex;not null"`
	ExpiresAt time.Time  `gorm:"not null;index:idx_refresh_tokens_user_expires,sort:desc"`
	RevokedAt *time.Time `gorm:"index"`
	CreatedAt time.Time  `gorm:"not null"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

func (t *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

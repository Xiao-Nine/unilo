package drop

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"unilo/internal/user"
)

type Drop struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
	AuthorID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	Author       user.User      `gorm:"foreignKey:AuthorID"`
	Content      string         `gorm:"type:text;not null"`
	LikeCount    int            `gorm:"not null;default:0"`
	CommentCount int            `gorm:"not null;default:0"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (Drop) TableName() string {
	return "drops"
}

func (d *Drop) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

type DropLike struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	DropID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_drop_likes_drop_user"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_drop_likes_drop_user;index"`
	CreatedAt time.Time `gorm:"not null"`
}

func (DropLike) TableName() string {
	return "drop_likes"
}

type DropComment struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey"`
	DropID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	Author        user.User      `gorm:"foreignKey:UserID"`
	ParentID      *uuid.UUID     `gorm:"type:uuid;index"`
	ReplyToUserID *uuid.UUID     `gorm:"type:uuid;index"`
	ReplyToUser   *user.User     `gorm:"foreignKey:ReplyToUserID"`
	Content       string         `gorm:"type:text;not null"`
	CreatedAt     time.Time      `gorm:"not null"`
	UpdatedAt     time.Time      `gorm:"not null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (DropComment) TableName() string {
	return "drop_comments"
}

func (c *DropComment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

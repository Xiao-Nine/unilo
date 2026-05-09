package agent

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"unilo/internal/user"
)

type Conversation struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	User      user.User      `gorm:"foreignKey:UserID"`
	Title     string         `gorm:"size:200;not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Message struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	ConversationID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Conversation   Conversation   `gorm:"foreignKey:ConversationID"`
	Role           string         `gorm:"size:20;not null"`
	Content        string         `gorm:"type:text;not null"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time      `gorm:"not null"`
}

type Run struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey"`
	ConversationID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	Conversation       Conversation   `gorm:"foreignKey:ConversationID"`
	UserID             uuid.UUID      `gorm:"type:uuid;not null;index"`
	User               user.User      `gorm:"foreignKey:UserID"`
	UserMessageID      uuid.UUID      `gorm:"type:uuid;not null;index"`
	UserMessage        Message        `gorm:"foreignKey:UserMessageID"`
	AssistantMessageID *uuid.UUID     `gorm:"type:uuid;index"`
	AssistantMessage   *Message       `gorm:"foreignKey:AssistantMessageID"`
	Status             string         `gorm:"size:20;not null;index"`
	Error              string         `gorm:"type:text"`
	Metadata           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time      `gorm:"not null"`
	StartedAt          *time.Time
	CompletedAt        *time.Time
	UpdatedAt          time.Time `gorm:"not null"`
}

func (Conversation) TableName() string {
	return "agent_conversations"
}

func (Message) TableName() string {
	return "agent_messages"
}

func (Run) TableName() string {
	return "agent_runs"
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (r *Run) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

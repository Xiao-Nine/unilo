package channel

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"unilo/internal/user"
)

type Channel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Name      string         `gorm:"size:100;not null"`
	CreatedBy uuid.UUID      `gorm:"type:uuid;not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Channel) TableName() string {
	return "channels"
}

func (c *Channel) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type Message struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`
	ChannelID uuid.UUID      `gorm:"type:uuid;not null;index:idx_channel_messages_channel_id_id"`
	SenderID  uuid.UUID      `gorm:"type:uuid;not null;index"`
	Sender    user.User      `gorm:"foreignKey:SenderID"`
	ReplyToID *int64         `gorm:"index"`
	MsgType   string         `gorm:"size:20;not null"`
	Content   string         `gorm:"type:text;not null"`
	Metadata  datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Message) TableName() string {
	return "channel_messages"
}

type ChannelRead struct {
	UserID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ChannelID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	LastReadMessageID int64     `gorm:"not null;default:0"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (ChannelRead) TableName() string {
	return "channel_reads"
}

package channel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"unilo/internal/user"
)

type ChannelResponse struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	CreatedBy         uuid.UUID `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	LastMessageID     int64     `json:"last_message_id"`
	LastReadMessageID int64     `json:"last_read_message_id"`
	UnreadCount       int64     `json:"unread_count"`
}

type CreateChannelRequest struct {
	Name string `json:"name"`
}

type UpdateChannelRequest struct {
	Name string `json:"name"`
}

type DeleteChannelResponse struct {
	Deleted bool `json:"deleted"`
}

type MarkChannelReadRequest struct {
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type ChannelReadResponse struct {
	ChannelID         uuid.UUID `json:"channel_id"`
	LastReadMessageID int64     `json:"last_read_message_id"`
	UnreadCount       int64     `json:"unread_count"`
}

type CreateMessageRequest struct {
	ReplyToID *int64          `json:"reply_to_id"`
	MsgType   string          `json:"msg_type"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
}

type MessageResponse struct {
	ID        int64           `json:"id"`
	ChannelID uuid.UUID       `json:"channel_id"`
	SenderID  uuid.UUID       `json:"sender_id"`
	Sender    user.DTO        `json:"sender"`
	ReplyToID *int64          `json:"reply_to_id"`
	MsgType   string          `json:"msg_type"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

type MessageListResponse struct {
	Messages   []MessageResponse `json:"messages"`
	NextCursor int64             `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

func ToChannelResponse(c Channel) ChannelResponse {
	return ChannelResponse{ID: c.ID, Name: c.Name, CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func ToChannelReadResponse(read ChannelRead) ChannelReadResponse {
	return ChannelReadResponse{ChannelID: read.ChannelID, LastReadMessageID: read.LastReadMessageID, UnreadCount: 0}
}

func ToMessageResponse(m Message) MessageResponse {
	metadata := json.RawMessage(m.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return MessageResponse{
		ID:        m.ID,
		ChannelID: m.ChannelID,
		SenderID:  m.SenderID,
		Sender:    user.ToSenderDTO(m.Sender),
		ReplyToID: m.ReplyToID,
		MsgType:   m.MsgType,
		Content:   m.Content,
		Metadata:  metadata,
		CreatedAt: m.CreatedAt,
	}
}

func JSONMetadata(raw json.RawMessage) datatypes.JSON {
	if len(raw) == 0 || string(raw) == "null" {
		return datatypes.JSON([]byte(`{}`))
	}
	return datatypes.JSON(raw)
}

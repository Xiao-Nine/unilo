package agent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CreateConversationRequest struct {
	Title string `json:"title"`
}

type ConversationResponse struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AgentContextRequest struct {
	Type      string     `json:"type"`
	ChannelID *uuid.UUID `json:"channel_id"`
}

type SendMessageRequest struct {
	Prompt  string              `json:"prompt"`
	Context AgentContextRequest `json:"context"`
}

type MessageResponse struct {
	ID        uuid.UUID       `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

type SendMessageResponse struct {
	Status           string          `json:"status"`
	ConversationID   uuid.UUID       `json:"conversation_id"`
	UserMessage      MessageResponse `json:"user_message"`
	AssistantMessage MessageResponse `json:"assistant_message"`
}

type ConversationListResponse struct {
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
	Items []ConversationResponse `json:"items"`
}

type MessageListResponse struct {
	Messages   []MessageResponse `json:"messages"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type SubmitRunResponse struct {
	RunID          uuid.UUID       `json:"run_id"`
	Status         string          `json:"status"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	UserMessage    MessageResponse `json:"user_message"`
}

type RunResponse struct {
	ID               uuid.UUID        `json:"id"`
	ConversationID   uuid.UUID        `json:"conversation_id"`
	Status           string           `json:"status"`
	UserMessage      MessageResponse  `json:"user_message"`
	AssistantMessage *MessageResponse `json:"assistant_message"`
	Error            string           `json:"error,omitempty"`
	Metadata         json.RawMessage  `json:"metadata"`
	CreatedAt        time.Time        `json:"created_at"`
	StartedAt        *time.Time       `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type AgentRunEvent struct {
	RunID            uuid.UUID        `json:"run_id"`
	ConversationID   uuid.UUID        `json:"conversation_id"`
	ChannelID        *uuid.UUID       `json:"channel_id,omitempty"`
	Status           string           `json:"status"`
	Content          string           `json:"content,omitempty"`
	Delta            string           `json:"delta,omitempty"`
	UserMessage      *MessageResponse `json:"user_message,omitempty"`
	AssistantMessage *MessageResponse `json:"assistant_message,omitempty"`
	Error            string           `json:"error,omitempty"`
	IsFinal          bool             `json:"is_final"`
}

func ToConversationResponse(c Conversation) ConversationResponse {
	return ConversationResponse{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func ToMessageResponse(m Message) MessageResponse {
	metadata := json.RawMessage(m.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return MessageResponse{ID: m.ID, Role: m.Role, Content: m.Content, Metadata: metadata, CreatedAt: m.CreatedAt}
}

func ToRunResponse(r Run) RunResponse {
	metadata := json.RawMessage(r.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	userMessage := ToMessageResponse(r.UserMessage)
	var assistantMessage *MessageResponse
	if r.AssistantMessage != nil {
		resp := ToMessageResponse(*r.AssistantMessage)
		assistantMessage = &resp
	}
	return RunResponse{
		ID:               r.ID,
		ConversationID:   r.ConversationID,
		Status:           r.Status,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Error:            r.Error,
		Metadata:         metadata,
		CreatedAt:        r.CreatedAt,
		StartedAt:        r.StartedAt,
		CompletedAt:      r.CompletedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

package realtime

import (
	"encoding/json"

	"github.com/google/uuid"
)

type IncomingMessage struct {
	Action    string          `json:"action"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type OutgoingMessage struct {
	Event     string `json:"event"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data"`
}

type ErrorData struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type sendMessagePayload struct {
	ChannelID uuid.UUID       `json:"channel_id"`
	ReplyToID *int64          `json:"reply_to_id"`
	MsgType   string          `json:"msg_type"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
}

type typingPayload struct {
	ChannelID uuid.UUID `json:"channel_id"`
	IsTyping  bool      `json:"is_typing"`
}

type invokeAgentPayload struct {
	ConversationID *uuid.UUID `json:"conversation_id"`
	ChannelID      *uuid.UUID `json:"channel_id"`
	Prompt         string     `json:"prompt"`
	ContextType    string     `json:"context_type"`
	Title          string     `json:"title"`
}

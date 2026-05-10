package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"unilo/internal/agent"
	"unilo/internal/channel"
	"unilo/pkg/apperror"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

type Client struct {
	id     uuid.UUID
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
}

func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg IncomingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.sendError("", 400, "invalid payload")
			continue
		}
		c.handleMessage(msg)
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg IncomingMessage) {
	switch msg.Action {
	case "ping":
		c.sendMessage(OutgoingMessage{Event: "pong", RequestID: msg.RequestID, Data: map[string]any{"ok": true}})
	case "send_message":
		c.handleSendMessage(msg)
	case "typing":
		c.handleTyping(msg)
	case "invoke_agent":
		c.handleInvokeAgent(msg)
	default:
		c.sendError(msg.RequestID, 400, "unknown action")
	}
}

func (c *Client) handleTyping(msg IncomingMessage) {
	var payload typingPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.ChannelID == uuid.Nil {
		c.sendError(msg.RequestID, 400, "invalid payload")
		return
	}
	c.hub.Broadcast("typing_status", map[string]any{"channel_id": payload.ChannelID, "user_id": c.userID, "is_typing": payload.IsTyping})
}

func (c *Client) handleSendMessage(msg IncomingMessage) {
	if c.hub.channels == nil {
		c.sendError(msg.RequestID, 500, "channel service is unavailable")
		return
	}
	var payload sendMessagePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.ChannelID == uuid.Nil {
		c.sendError(msg.RequestID, 400, "invalid payload")
		return
	}
	resp, err := c.hub.channels.CreateMessage(payload.ChannelID, c.userID, channel.CreateMessageRequest{
		ReplyToID: payload.ReplyToID,
		MsgType:   payload.MsgType,
		Content:   payload.Content,
		Metadata:  payload.Metadata,
	})
	if err != nil {
		appErr := apperror.From(err)
		c.sendError(msg.RequestID, appErr.Code, appErr.Message)
		return
	}
	c.sendMessage(OutgoingMessage{Event: "message_sent", RequestID: msg.RequestID, Data: resp})
}

func (c *Client) handleInvokeAgent(msg IncomingMessage) {
	if c.hub.agents == nil {
		c.sendError(msg.RequestID, 500, "agent service is unavailable")
		return
	}
	var payload invokeAgentPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, 400, "invalid payload")
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		c.sendError(msg.RequestID, 400, "prompt is required")
		return
	}
	contextType := strings.TrimSpace(payload.ContextType)
	if contextType == "" {
		contextType = "global"
	}
	if contextType == "channel" && (payload.ChannelID == nil || *payload.ChannelID == uuid.Nil) {
		c.sendError(msg.RequestID, 400, "channel_id is required")
		return
	}
	conversationID := uuid.Nil
	if payload.ConversationID != nil && *payload.ConversationID != uuid.Nil {
		conversationID = *payload.ConversationID
	} else {
		title := strings.TrimSpace(payload.Title)
		if title == "" {
			title = prompt
			if len([]rune(title)) > 40 {
				title = string([]rune(title)[:40])
			}
		}
		conversation, err := c.hub.agents.CreateConversation(c.userID, agent.CreateConversationRequest{Title: title})
		if err != nil {
			appErr := apperror.From(err)
			c.sendError(msg.RequestID, appErr.Code, appErr.Message)
			return
		}
		conversationID = conversation.ID
	}
	req := agent.SendMessageRequest{Prompt: prompt, Context: agent.AgentContextRequest{Type: contextType, ChannelID: payload.ChannelID}}
	resp, err := c.hub.agents.SubmitRun(context.Background(), c.userID, conversationID, req)
	if err != nil {
		appErr := apperror.From(err)
		c.sendError(msg.RequestID, appErr.Code, appErr.Message)
		return
	}
	c.hub.SendToUser(c.userID, "agent_message", msg.RequestID, agent.AgentRunEvent{
		RunID:          resp.RunID,
		ConversationID: resp.ConversationID,
		ChannelID:      payload.ChannelID,
		Status:         resp.Status,
		UserMessage:    &resp.UserMessage,
	})
}

func (c *Client) sendError(requestID string, code int, message string) {
	c.sendMessage(OutgoingMessage{Event: "error", RequestID: requestID, Data: ErrorData{Code: code, Msg: message}})
}

func (c *Client) sendMessage(msg OutgoingMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
		c.hub.unregister <- c
	}
}

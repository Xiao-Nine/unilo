package realtime

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"unilo/internal/agent"
)

type Hub struct {
	log         *slog.Logger
	clients     map[*Client]bool
	userClients map[uuid.UUID]map[*Client]bool
	register    chan *Client
	unregister  chan *Client
	broadcast   chan OutgoingMessage
	channels    ChannelService
	agents      AgentService
	broker      Broker
	presence    Presence
}

func NewHub(log *slog.Logger, broker Broker, presence Presence) *Hub {
	return &Hub{
		log:         log,
		clients:     make(map[*Client]bool),
		userClients: make(map[uuid.UUID]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan OutgoingMessage, 256),
		broker:      broker,
		presence:    presence,
	}
}

func (h *Hub) SetChannelService(service ChannelService) {
	h.channels = service
}

func (h *Hub) SetAgentService(service AgentService) {
	h.agents = service
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case msg := <-h.broadcast:
			h.deliver(msg)
		}
	}
}

func (h *Hub) Subscribe(ctx context.Context) {
	if h.broker == nil {
		return
	}
	if err := h.broker.Subscribe(ctx, func(env Envelope) {
		var data any
		if len(env.Data) > 0 {
			data = json.RawMessage(env.Data)
		}
		h.deliverEnvelope(env, data)
	}); err != nil && ctx.Err() == nil {
		h.log.Error("realtime broker subscribe failed", "error", err)
	}
}

func (h *Hub) Broadcast(event string, data any) {
	h.publishOrDeliver(Envelope{Event: event}, data)
}

func (h *Hub) SendToUser(userID uuid.UUID, event string, requestID string, data any) {
	h.publishOrDeliver(Envelope{Event: event, RequestID: requestID, TargetUserID: &userID}, data)
}

func (h *Hub) NotifyAgentRun(userID uuid.UUID, event agent.AgentRunEvent) {
	eventName := "agent_message"
	if event.Delta != "" {
		eventName = "agent_delta"
	}
	h.SendToUser(userID, eventName, "", event)
}

func (h *Hub) RefreshPresence(client *Client) {
	if h.presence != nil {
		_ = h.presence.Refresh(context.Background(), client.userID, client.id)
	}
}

func (h *Hub) registerClient(client *Client) {
	h.clients[client] = true
	if h.userClients[client.userID] == nil {
		h.userClients[client.userID] = make(map[*Client]bool)
	}
	h.userClients[client.userID][client] = true
	if h.presence != nil {
		h.sendPresenceSnapshot(client)
		online, err := h.presence.Register(context.Background(), client.userID, client.id)
		if err != nil {
			h.log.Error("register presence failed", "error", err)
		}
		if online {
			h.Broadcast("presence_updated", map[string]any{"user_id": client.userID, "status": "online"})
		}
		return
	}
	h.Broadcast("presence_updated", map[string]any{"user_id": client.userID, "status": "online"})
}

func (h *Hub) sendPresenceSnapshot(client *Client) {
	userIDs, err := h.presence.OnlineUsers(context.Background())
	if err != nil {
		h.log.Error("load presence snapshot failed", "error", err)
		return
	}
	for _, userID := range userIDs {
		client.sendMessage(OutgoingMessage{Event: "presence_updated", Data: map[string]any{"user_id": userID, "status": "online"}})
	}
}

func (h *Hub) unregisterClient(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	if clients := h.userClients[client.userID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.userClients, client.userID)
		}
	}
	close(client.send)
	if h.presence != nil {
		offline, err := h.presence.Unregister(context.Background(), client.userID, client.id)
		if err != nil {
			h.log.Error("unregister presence failed", "error", err)
		}
		if offline {
			h.Broadcast("presence_updated", map[string]any{"user_id": client.userID, "status": "offline"})
		}
		return
	}
	if h.userClients[client.userID] == nil {
		h.Broadcast("presence_updated", map[string]any{"user_id": client.userID, "status": "offline"})
	}
}

func (h *Hub) publishOrDeliver(env Envelope, data any) {
	if h.broker != nil {
		if err := h.broker.Publish(context.Background(), env.withData(data)); err != nil {
			h.log.Error("publish realtime event failed", "error", err)
		}
		return
	}
	h.deliverEnvelope(env, data)
}

func (h *Hub) deliver(msg OutgoingMessage) {
	h.deliverTo(nil, msg)
}

func (h *Hub) deliverEnvelope(env Envelope, data any) {
	h.deliverTo(env.TargetUserID, OutgoingMessage{Event: env.Event, RequestID: env.RequestID, Data: data})
}

func (h *Hub) deliverTo(targetUserID *uuid.UUID, msg OutgoingMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("marshal websocket event failed", "error", err)
		return
	}
	if targetUserID != nil {
		for client := range h.userClients[*targetUserID] {
			h.sendToClient(client, payload)
		}
		return
	}
	for client := range h.clients {
		h.sendToClient(client, payload)
	}
}

func (h *Hub) sendToClient(client *Client, payload []byte) {
	select {
	case client.send <- payload:
	default:
		delete(h.clients, client)
		if clients := h.userClients[client.userID]; clients != nil {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.userClients, client.userID)
			}
		}
		close(client.send)
	}
}

package realtime

import (
	"context"

	"github.com/google/uuid"

	"unilo/internal/agent"
	"unilo/internal/channel"
)

type ChannelService interface {
	CreateMessage(channelID uuid.UUID, senderID uuid.UUID, req channel.CreateMessageRequest) (channel.MessageResponse, error)
}

type AgentService interface {
	CreateConversation(userID uuid.UUID, req agent.CreateConversationRequest) (agent.ConversationResponse, error)
	SubmitRun(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, req agent.SendMessageRequest) (agent.SubmitRunResponse, error)
}

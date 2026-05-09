package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"unilo/internal/config"
	"unilo/internal/search"
	"unilo/pkg/apperror"
)

const maxPromptLength = 20000

const (
	RunStatusQueued    = "queued"
	RunStatusRunning   = "running"
	RunStatusStreaming = "streaming"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

type RetrievalService interface {
	Search(query string, sourceType string, limit int) (search.SearchResponse, error)
	SearchWithOptions(opts search.SearchOptions) (search.SearchResponse, error)
}

type LLMClient interface {
	Complete(ctx context.Context, messages []ChatMessage) (CompleteResult, error)
	StreamComplete(ctx context.Context, messages []ChatMessage, onDelta StreamDelta) (CompleteResult, error)
}

type RunNotifier interface {
	NotifyAgentRun(userID uuid.UUID, event AgentRunEvent)
}

type Service struct {
	repo      *Repository
	retrieval RetrievalService
	client    LLMClient
	cfg       config.AgentConfig
	notifier  RunNotifier
	wakeRuns  chan struct{}
}

func NewService(repo *Repository, retrieval RetrievalService, client LLMClient, cfg config.AgentConfig) *Service {
	return &Service{repo: repo, retrieval: retrieval, client: client, cfg: cfg, wakeRuns: make(chan struct{}, 1)}
}

func (s *Service) SetNotifier(notifier RunNotifier) {
	s.notifier = notifier
}

func (s *Service) CreateConversation(userID uuid.UUID, req CreateConversationRequest) (ConversationResponse, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 200 {
		return ConversationResponse{}, apperror.BadRequest("title is too long")
	}
	conversation := Conversation{UserID: userID, Title: title}
	if err := s.repo.CreateConversation(&conversation); err != nil {
		return ConversationResponse{}, apperror.Internal(err)
	}
	return ToConversationResponse(conversation), nil
}

func (s *Service) ListConversations(userID uuid.UUID, page int, size int) (ConversationListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	conversations, total, err := s.repo.ListConversationsForUser(userID, page, size)
	if err != nil {
		return ConversationListResponse{}, apperror.Internal(err)
	}
	items := make([]ConversationResponse, 0, len(conversations))
	for _, conversation := range conversations {
		items = append(items, ToConversationResponse(conversation))
	}
	return ConversationListResponse{Total: total, Page: page, Size: size, Items: items}, nil
}

func (s *Service) ListMessages(userID uuid.UUID, conversationID uuid.UUID, cursor string, limit int) (MessageListResponse, error) {
	conversation, err := s.repo.FindConversationForUser(conversationID, userID)
	if err != nil {
		if IsNotFound(err) {
			return MessageListResponse{}, apperror.NotFound("conversation is not found")
		}
		return MessageListResponse{}, apperror.Internal(err)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var before *time.Time
	if strings.TrimSpace(cursor) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cursor))
		if err != nil {
			return MessageListResponse{}, apperror.BadRequest("cursor is invalid")
		}
		before = &parsed
	}

	messages, err := s.repo.ListMessages(conversation.ID, before, limit+1)
	if err != nil {
		return MessageListResponse{}, apperror.Internal(err)
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	resp := make([]MessageResponse, 0, len(messages))
	nextCursor := ""
	if len(messages) > 0 {
		nextCursor = messages[0].CreatedAt.Format(time.RFC3339Nano)
	}
	for _, message := range messages {
		resp = append(resp, ToMessageResponse(message))
	}
	return MessageListResponse{Messages: resp, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) SendMessage(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, req SendMessageRequest) (SendMessageResponse, error) {
	if err := s.ensureAgentReady(); err != nil {
		return SendMessageResponse{}, err
	}
	conversation, err := s.findConversationForUser(conversationID, userID)
	if err != nil {
		return SendMessageResponse{}, err
	}
	prompt, err := validatePrompt(req.Prompt)
	if err != nil {
		return SendMessageResponse{}, err
	}
	contextMetadata := normalizeContext(req.Context)
	metadataBytes, _ := json.Marshal(map[string]any{"context": contextMetadata})
	userMessage := Message{ConversationID: conversation.ID, Role: "user", Content: prompt, Metadata: datatypes.JSON(metadataBytes)}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateMessage(tx, &userMessage); err != nil {
			return err
		}
		return s.repo.TouchConversation(tx, conversation.ID)
	}); err != nil {
		return SendMessageResponse{}, apperror.Internal(err)
	}

	retrieved, err := s.retrieve(prompt, contextMetadata)
	if err != nil {
		return SendMessageResponse{}, err
	}
	history, err := s.repo.ListRecentMessages(conversation.ID, s.cfg.MaxHistoryMessages)
	if err != nil {
		return SendMessageResponse{}, apperror.Internal(err)
	}
	result, err := s.client.Complete(ctx, buildModelMessages(history, retrieved))
	if err != nil {
		return SendMessageResponse{}, err
	}
	assistantMessage, err := s.persistAssistantMessage(conversation.ID, result, retrieved, contextMetadata)
	if err != nil {
		return SendMessageResponse{}, err
	}
	return SendMessageResponse{Status: RunStatusCompleted, ConversationID: conversation.ID, UserMessage: ToMessageResponse(userMessage), AssistantMessage: ToMessageResponse(assistantMessage)}, nil
}

func (s *Service) SubmitRun(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID, req SendMessageRequest) (SubmitRunResponse, error) {
	if err := s.ensureAgentReady(); err != nil {
		return SubmitRunResponse{}, err
	}
	conversation, err := s.findConversationForUser(conversationID, userID)
	if err != nil {
		return SubmitRunResponse{}, err
	}
	prompt, err := validatePrompt(req.Prompt)
	if err != nil {
		return SubmitRunResponse{}, err
	}
	contextMetadata := normalizeContext(req.Context)
	metadataBytes, _ := json.Marshal(map[string]any{"context": contextMetadata})
	userMessage := Message{ConversationID: conversation.ID, Role: "user", Content: prompt, Metadata: datatypes.JSON(metadataBytes)}
	run := Run{ConversationID: conversation.ID, UserID: userID, Status: RunStatusQueued, Metadata: datatypes.JSON(metadataBytes)}
	if err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateMessage(tx, &userMessage); err != nil {
			return err
		}
		run.UserMessageID = userMessage.ID
		if err := s.repo.CreateRun(tx, &run); err != nil {
			return err
		}
		return s.repo.TouchConversation(tx, conversation.ID)
	}); err != nil {
		return SubmitRunResponse{}, apperror.Internal(err)
	}
	s.wakeWorker()
	return SubmitRunResponse{RunID: run.ID, Status: run.Status, ConversationID: conversation.ID, UserMessage: ToMessageResponse(userMessage)}, nil
}

func (s *Service) GetRun(userID uuid.UUID, runID uuid.UUID) (RunResponse, error) {
	run, err := s.repo.FindRunForUser(runID, userID)
	if err != nil {
		if IsNotFound(err) {
			return RunResponse{}, apperror.NotFound("run is not found")
		}
		return RunResponse{}, apperror.Internal(err)
	}
	return ToRunResponse(run), nil
}

func (s *Service) StartWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		s.processQueuedRuns(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.wakeRuns:
		case <-ticker.C:
		}
	}
}

func (s *Service) processQueuedRuns(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		run, err := s.repo.ClaimNextQueuedRun()
		if err != nil {
			if IsNotFound(err) {
				return
			}
			return
		}
		s.processRun(ctx, run)
	}
}

func (s *Service) processRun(ctx context.Context, run Run) {
	run, err := s.repo.FindRun(run.ID)
	if err != nil {
		return
	}
	s.notify(run.UserID, AgentRunEvent{RunID: run.ID, ConversationID: run.ConversationID, ChannelID: channelIDFromMetadata(run.Metadata), Status: RunStatusRunning, UserMessage: messagePtr(ToMessageResponse(run.UserMessage))})

	contextMetadata := contextFromMetadata(run.Metadata)
	retrieved, err := s.retrieve(run.UserMessage.Content, contextMetadata)
	if err != nil {
		s.failRun(run, err)
		return
	}
	history, err := s.repo.ListRecentMessages(run.ConversationID, s.cfg.MaxHistoryMessages)
	if err != nil {
		s.failRun(run, apperror.Internal(err))
		return
	}
	modelMessages := buildModelMessages(history, retrieved)
	streamingStarted := false
	workerCtx, cancel := context.WithTimeout(ctx, s.workerTimeout())
	defer cancel()
	result, err := s.client.StreamComplete(workerCtx, modelMessages, func(delta string) error {
		if !streamingStarted {
			streamingStarted = true
			_ = s.repo.UpdateRun(nil, run.ID, map[string]any{"status": RunStatusStreaming})
			s.notify(run.UserID, AgentRunEvent{RunID: run.ID, ConversationID: run.ConversationID, ChannelID: channelIDFromMetadata(run.Metadata), Status: RunStatusStreaming})
		}
		s.notify(run.UserID, AgentRunEvent{RunID: run.ID, ConversationID: run.ConversationID, ChannelID: channelIDFromMetadata(run.Metadata), Status: RunStatusStreaming, Delta: delta})
		return nil
	})
	if err != nil {
		s.failRun(run, err)
		return
	}
	assistantMessage, metadata, err := s.persistAssistantMessageWithMetadata(run.ConversationID, result, retrieved, contextMetadata)
	if err != nil {
		s.failRun(run, err)
		return
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		return s.repo.AttachAssistantMessage(tx, run.ID, assistantMessage.ID, metadata)
	}); err != nil {
		s.failRun(run, apperror.Internal(err))
		return
	}
	assistantResp := ToMessageResponse(assistantMessage)
	s.notify(run.UserID, AgentRunEvent{RunID: run.ID, ConversationID: run.ConversationID, ChannelID: channelIDFromMetadata(run.Metadata), Status: RunStatusCompleted, Content: assistantMessage.Content, AssistantMessage: &assistantResp, IsFinal: true})
}

func (s *Service) failRun(run Run, err error) {
	appErr := apperror.From(err)
	now := time.Now()
	_ = s.repo.UpdateRun(nil, run.ID, map[string]any{"status": RunStatusFailed, "error": appErr.Message, "completed_at": now})
	s.notify(run.UserID, AgentRunEvent{RunID: run.ID, ConversationID: run.ConversationID, ChannelID: channelIDFromMetadata(run.Metadata), Status: RunStatusFailed, Error: appErr.Message, IsFinal: true})
}

func (s *Service) persistAssistantMessage(conversationID uuid.UUID, result CompleteResult, retrieved search.SearchResponse, contextMetadata map[string]any) (Message, error) {
	assistantMessage, _, err := s.persistAssistantMessageWithMetadata(conversationID, result, retrieved, contextMetadata)
	return assistantMessage, err
}

func (s *Service) persistAssistantMessageWithMetadata(conversationID uuid.UUID, result CompleteResult, retrieved search.SearchResponse, contextMetadata map[string]any) (Message, datatypes.JSON, error) {
	assistantMetadata, _ := json.Marshal(map[string]any{
		"model":     result.Model,
		"usage":     result.Usage,
		"citations": retrieved.Items,
		"context":   contextMetadata,
	})
	metadata := datatypes.JSON(assistantMetadata)
	assistantMessage := Message{ConversationID: conversationID, Role: "assistant", Content: result.Content, Metadata: metadata}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateMessage(tx, &assistantMessage); err != nil {
			return err
		}
		return s.repo.TouchConversation(tx, conversationID)
	}); err != nil {
		return Message{}, nil, apperror.Internal(err)
	}
	return assistantMessage, metadata, nil
}

func (s *Service) retrieve(prompt string, contextMetadata map[string]any) (search.SearchResponse, error) {
	opts := search.SearchOptions{Query: prompt, SourceType: "all", Limit: s.cfg.MaxContextResults}
	if contextMetadata["type"] == "channel" {
		if raw, ok := contextMetadata["channel_id"].(string); ok && raw != "" {
			if channelID, err := uuid.Parse(raw); err == nil && channelID != uuid.Nil {
				opts.SourceType = "messages"
				opts.ChannelID = &channelID
			}
		}
	}
	return s.retrieval.SearchWithOptions(opts)
}

func (s *Service) ensureAgentReady() error {
	if !s.cfg.Enabled {
		return apperror.BadRequest("agent is disabled")
	}
	if s.client == nil {
		return apperror.New(500, "agent provider is not configured")
	}
	return nil
}

func (s *Service) findConversationForUser(conversationID uuid.UUID, userID uuid.UUID) (Conversation, error) {
	conversation, err := s.repo.FindConversationForUser(conversationID, userID)
	if err != nil {
		if IsNotFound(err) {
			return Conversation{}, apperror.NotFound("conversation is not found")
		}
		return Conversation{}, apperror.Internal(err)
	}
	return conversation, nil
}

func (s *Service) wakeWorker() {
	select {
	case s.wakeRuns <- struct{}{}:
	default:
	}
}

func (s *Service) workerTimeout() time.Duration {
	if s.cfg.Timeout > 0 {
		return s.cfg.Timeout
	}
	return 30 * time.Second
}

func (s *Service) notify(userID uuid.UUID, event AgentRunEvent) {
	if s.notifier != nil {
		s.notifier.NotifyAgentRun(userID, event)
	}
}

func validatePrompt(raw string) (string, error) {
	prompt := strings.TrimSpace(raw)
	if prompt == "" {
		return "", apperror.BadRequest("prompt is required")
	}
	if len([]rune(prompt)) > maxPromptLength {
		return "", apperror.BadRequest("prompt is too long")
	}
	return prompt, nil
}

func normalizeContext(ctx AgentContextRequest) map[string]any {
	contextType := strings.TrimSpace(ctx.Type)
	if contextType == "" {
		contextType = "global"
	}
	metadata := map[string]any{"type": contextType}
	if ctx.ChannelID != nil {
		metadata["channel_id"] = ctx.ChannelID.String()
	}
	return metadata
}

func contextFromMetadata(metadata datatypes.JSON) map[string]any {
	var raw struct {
		Context map[string]any `json:"context"`
	}
	if len(metadata) == 0 || json.Unmarshal(metadata, &raw) != nil || raw.Context == nil {
		return map[string]any{"type": "global"}
	}
	return raw.Context
}

func channelIDFromMetadata(metadata datatypes.JSON) *uuid.UUID {
	ctx := contextFromMetadata(metadata)
	raw, ok := ctx["channel_id"].(string)
	if !ok || raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}

func messagePtr(message MessageResponse) *MessageResponse {
	return &message
}

func buildModelMessages(history []Message, retrieved search.SearchResponse) []ChatMessage {
	messages := []ChatMessage{
		{Role: "system", Content: "You are Unilo's assistant. Answer using the provided Unilo context when relevant. If the context does not contain enough information, say so clearly. Be concise and practical."},
		{Role: "system", Content: buildRetrievalContext(retrieved)},
	}
	for _, msg := range history {
		if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "system" {
			continue
		}
		messages = append(messages, ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	return messages
}

func buildRetrievalContext(retrieved search.SearchResponse) string {
	if len(retrieved.Items) == 0 {
		return "Relevant Unilo search results: none."
	}
	var b strings.Builder
	b.WriteString("Relevant Unilo search results:\n")
	for i, item := range retrieved.Items {
		_, _ = fmt.Fprintf(&b, "[%d] type=%s id=%s title=%s snippet=%s\n", i+1, item.Type, item.ID, item.Title, item.Snippet)
	}
	return strings.TrimSpace(b.String())
}

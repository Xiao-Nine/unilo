package channel

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"unilo/pkg/apperror"
)

type Notifier interface {
	Broadcast(event string, data any)
	SendToUser(userID uuid.UUID, event string, requestID string, data any)
}

type Indexer interface {
	UpsertDocument(tx *gorm.DB, sourceType string, sourceID string, title string, content string, metadata map[string]any) error
}

type noopNotifier struct{}

func (noopNotifier) Broadcast(string, any) {}

func (noopNotifier) SendToUser(uuid.UUID, string, string, any) {}

type noopIndexer struct{}

func (noopIndexer) UpsertDocument(*gorm.DB, string, string, string, string, map[string]any) error {
	return nil
}

type Service struct {
	repo     *Repository
	notifier Notifier
	indexer  Indexer
}

func NewService(repo *Repository, notifier Notifier, indexer Indexer) *Service {
	if notifier == nil {
		notifier = noopNotifier{}
	}
	if indexer == nil {
		indexer = noopIndexer{}
	}
	return &Service{repo: repo, notifier: notifier, indexer: indexer}
}

func (s *Service) ListChannels(userID uuid.UUID) ([]ChannelResponse, error) {
	channels, err := s.repo.ListChannelsWithReadState(userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	resp := make([]ChannelResponse, 0, len(channels))
	for _, c := range channels {
		item := ToChannelResponse(c.Channel)
		item.LastMessageID = c.LastMessageID
		item.LastReadMessageID = c.LastReadMessageID
		item.UnreadCount = c.UnreadCount
		resp = append(resp, item)
	}
	return resp, nil
}

func (s *Service) CreateChannel(userID uuid.UUID, req CreateChannelRequest) (ChannelResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return ChannelResponse{}, apperror.BadRequest("channel name length must be between 1 and 100")
	}

	ch := Channel{Name: name, CreatedBy: userID}
	if err := s.repo.CreateChannel(&ch); err != nil {
		return ChannelResponse{}, apperror.Internal(err)
	}
	resp := ToChannelResponse(ch)
	s.notifier.Broadcast("channel_created", resp)
	return resp, nil
}

func (s *Service) UpdateChannel(channelID uuid.UUID, req UpdateChannelRequest) (ChannelResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return ChannelResponse{}, apperror.BadRequest("channel name length must be between 1 and 100")
	}

	ch, err := s.repo.FindChannel(channelID)
	if err != nil {
		if IsNotFound(err) {
			return ChannelResponse{}, apperror.NotFound("channel is not found")
		}
		return ChannelResponse{}, apperror.Internal(err)
	}
	ch.Name = name
	if err := s.repo.SaveChannel(&ch); err != nil {
		return ChannelResponse{}, apperror.Internal(err)
	}
	resp := ToChannelResponse(ch)
	s.notifier.Broadcast("channel_updated", resp)
	return resp, nil
}

func (s *Service) DeleteChannel(channelID uuid.UUID) (DeleteChannelResponse, error) {
	ch, err := s.repo.FindChannel(channelID)
	if err != nil {
		if IsNotFound(err) {
			return DeleteChannelResponse{}, apperror.NotFound("channel is not found")
		}
		return DeleteChannelResponse{}, apperror.Internal(err)
	}
	if err := s.repo.DeleteChannel(&ch); err != nil {
		return DeleteChannelResponse{}, apperror.Internal(err)
	}
	s.notifier.Broadcast("channel_deleted", map[string]any{"id": channelID})
	return DeleteChannelResponse{Deleted: true}, nil
}

func (s *Service) MarkChannelRead(userID uuid.UUID, channelID uuid.UUID, req MarkChannelReadRequest) (ChannelReadResponse, error) {
	if req.LastReadMessageID < 0 {
		return ChannelReadResponse{}, apperror.BadRequest("last_read_message_id is invalid")
	}
	if _, err := s.repo.FindChannel(channelID); err != nil {
		if IsNotFound(err) {
			return ChannelReadResponse{}, apperror.NotFound("channel is not found")
		}
		return ChannelReadResponse{}, apperror.Internal(err)
	}
	if req.LastReadMessageID > 0 {
		exists, err := s.repo.MessageExistsInChannel(channelID, req.LastReadMessageID)
		if err != nil {
			return ChannelReadResponse{}, apperror.Internal(err)
		}
		if !exists {
			return ChannelReadResponse{}, apperror.BadRequest("last_read_message_id is invalid")
		}
	}
	read, err := s.repo.MarkChannelRead(userID, channelID, req.LastReadMessageID)
	if err != nil {
		return ChannelReadResponse{}, apperror.Internal(err)
	}
	unreadCount, err := s.repo.CountUnreadMessages(userID, channelID, read.LastReadMessageID)
	if err != nil {
		return ChannelReadResponse{}, apperror.Internal(err)
	}
	resp := ToChannelReadResponse(read)
	resp.UnreadCount = unreadCount
	s.notifier.SendToUser(userID, "channel_read_updated", "", resp)
	return resp, nil
}

func (s *Service) CreateMessage(channelID uuid.UUID, senderID uuid.UUID, req CreateMessageRequest) (MessageResponse, error) {
	if _, err := s.repo.FindChannel(channelID); err != nil {
		if IsNotFound(err) {
			return MessageResponse{}, apperror.NotFound("channel is not found")
		}
		return MessageResponse{}, apperror.Internal(err)
	}
	if !validMessageType(req.MsgType) {
		return MessageResponse{}, apperror.BadRequest("msg_type is invalid")
	}
	if strings.TrimSpace(req.Content) == "" {
		return MessageResponse{}, apperror.BadRequest("content is required")
	}
	if req.ReplyToID != nil {
		var count int64
		err := s.repo.DB().Model(&Message{}).Where("id = ? AND channel_id = ?", *req.ReplyToID, channelID).Count(&count).Error
		if err != nil {
			return MessageResponse{}, apperror.Internal(err)
		}
		if count == 0 {
			return MessageResponse{}, apperror.BadRequest("reply_to_id is invalid")
		}
	}

	msg := Message{ChannelID: channelID, SenderID: senderID, ReplyToID: req.ReplyToID, MsgType: req.MsgType, Content: req.Content, Metadata: JSONMetadata(req.Metadata)}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		if err := tx.Preload("Sender").First(&msg, "id = ?", msg.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO channel_reads (user_id, channel_id, last_read_message_id, created_at, updated_at)
			VALUES (?, ?, ?, now(), now())
			ON CONFLICT (user_id, channel_id)
			DO UPDATE SET
				last_read_message_id = GREATEST(channel_reads.last_read_message_id, EXCLUDED.last_read_message_id),
				updated_at = now()
		`, senderID, channelID, msg.ID).Error; err != nil {
			return err
		}
		return s.indexer.UpsertDocument(tx, "message", strconv.FormatInt(msg.ID, 10), "频道消息", msg.Content, map[string]any{
			"channel_id": msg.ChannelID,
			"sender_id":  msg.SenderID,
			"msg_type":   msg.MsgType,
		})
	}); err != nil {
		return MessageResponse{}, apperror.Internal(err)
	}
	resp := ToMessageResponse(msg)
	s.notifier.Broadcast("new_message", resp)
	return resp, nil
}

func (s *Service) ListMessages(channelID uuid.UUID, cursor int64, limit int) (MessageListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if _, err := s.repo.FindChannel(channelID); err != nil {
		if IsNotFound(err) {
			return MessageListResponse{}, apperror.NotFound("channel is not found")
		}
		return MessageListResponse{}, apperror.Internal(err)
	}

	var messages []Message
	query := s.repo.DB().Preload("Sender").Where("channel_id = ?", channelID)
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	if err := query.Order("id DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
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
	nextCursor := int64(0)
	for _, msg := range messages {
		if nextCursor == 0 || msg.ID < nextCursor {
			nextCursor = msg.ID
		}
		resp = append(resp, ToMessageResponse(msg))
	}
	return MessageListResponse{Messages: resp, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func validMessageType(msgType string) bool {
	switch msgType {
	case "text", "code", "image", "file", "agent":
		return true
	default:
		return false
	}
}

func ParseChannelID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("channel_id is invalid")
	}
	return id, nil
}

func IsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

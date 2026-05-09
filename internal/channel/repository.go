package channel

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type ChannelWithReadState struct {
	Channel
	LastMessageID     int64
	LastReadMessageID int64
	UnreadCount       int64
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) ListChannels() ([]Channel, error) {
	var channels []Channel
	err := r.db.Order("created_at ASC").Find(&channels).Error
	return channels, err
}

func (r *Repository) ListChannelsWithReadState(userID uuid.UUID) ([]ChannelWithReadState, error) {
	var channels []ChannelWithReadState
	err := r.db.Raw(`
		SELECT
			c.id,
			c.name,
			c.created_by,
			c.created_at,
			c.updated_at,
			c.deleted_at,
			COALESCE((
				SELECT MAX(m.id)
				FROM channel_messages m
				WHERE m.channel_id = c.id AND m.deleted_at IS NULL
			), 0) AS last_message_id,
			COALESCE(cr.last_read_message_id, 0) AS last_read_message_id,
			COALESCE((
				SELECT COUNT(*)
				FROM channel_messages m
				WHERE m.channel_id = c.id
					AND m.deleted_at IS NULL
					AND m.sender_id <> ?
					AND m.id > COALESCE(cr.last_read_message_id, 0)
			), 0) AS unread_count
		FROM channels c
		LEFT JOIN channel_reads cr ON cr.channel_id = c.id AND cr.user_id = ?
		WHERE c.deleted_at IS NULL
		ORDER BY c.created_at ASC
	`, userID, userID).Scan(&channels).Error
	return channels, err
}

func (r *Repository) FindChannel(id uuid.UUID) (Channel, error) {
	var c Channel
	err := r.db.First(&c, "id = ?", id).Error
	return c, err
}

func (r *Repository) CreateChannel(c *Channel) error {
	return r.db.Create(c).Error
}

func (r *Repository) SaveChannel(c *Channel) error {
	return r.db.Save(c).Error
}

func (r *Repository) DeleteChannel(c *Channel) error {
	return r.db.Delete(c).Error
}

func (r *Repository) CreateMessage(m *Message) error {
	return r.db.Create(m).Error
}

func (r *Repository) MessageExistsInChannel(channelID uuid.UUID, messageID int64) (bool, error) {
	var count int64
	err := r.db.Model(&Message{}).Where("channel_id = ? AND id = ?", channelID, messageID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) MarkChannelRead(userID uuid.UUID, channelID uuid.UUID, lastReadMessageID int64) (ChannelRead, error) {
	err := r.db.Exec(`
		INSERT INTO channel_reads (user_id, channel_id, last_read_message_id, created_at, updated_at)
		VALUES (?, ?, ?, now(), now())
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			last_read_message_id = GREATEST(channel_reads.last_read_message_id, EXCLUDED.last_read_message_id),
			updated_at = now()
	`, userID, channelID, lastReadMessageID).Error
	if err != nil {
		return ChannelRead{}, err
	}

	var read ChannelRead
	err = r.db.First(&read, "user_id = ? AND channel_id = ?", userID, channelID).Error
	return read, err
}

func (r *Repository) CountUnreadMessages(userID uuid.UUID, channelID uuid.UUID, lastReadMessageID int64) (int64, error) {
	var count int64
	err := r.db.Model(&Message{}).
		Where("channel_id = ? AND deleted_at IS NULL AND sender_id <> ? AND id > ?", channelID, userID, lastReadMessageID).
		Count(&count).Error
	return count, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

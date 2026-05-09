package agent

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) CreateConversation(conversation *Conversation) error {
	return r.db.Create(conversation).Error
}

func (r *Repository) FindConversationForUser(conversationID uuid.UUID, userID uuid.UUID) (Conversation, error) {
	var conversation Conversation
	err := r.db.Where("id = ? AND user_id = ?", conversationID, userID).First(&conversation).Error
	return conversation, err
}

func (r *Repository) CreateMessage(tx *gorm.DB, message *Message) error {
	return tx.Create(message).Error
}

func (r *Repository) ListRecentMessages(conversationID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 {
		return nil, nil
	}
	var messages []Message
	err := r.db.Where("conversation_id = ?", conversationID).Order("created_at DESC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (r *Repository) ListConversationsForUser(userID uuid.UUID, page int, size int) ([]Conversation, int64, error) {
	var total int64
	query := r.db.Model(&Conversation{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var conversations []Conversation
	offset := (page - 1) * size
	err := r.db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Order("created_at DESC").
		Limit(size).
		Offset(offset).
		Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}
	return conversations, total, nil
}

func (r *Repository) ListMessages(conversationID uuid.UUID, before *time.Time, limit int) ([]Message, error) {
	var messages []Message
	query := r.db.Where("conversation_id = ?", conversationID)
	if before != nil {
		query = query.Where("created_at < ?", *before)
	}
	err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *Repository) CreateRun(tx *gorm.DB, run *Run) error {
	return tx.Create(run).Error
}

func (r *Repository) FindRunForUser(runID uuid.UUID, userID uuid.UUID) (Run, error) {
	var run Run
	err := r.db.Preload("UserMessage").Preload("AssistantMessage").Where("id = ? AND user_id = ?", runID, userID).First(&run).Error
	return run, err
}

func (r *Repository) FindRun(runID uuid.UUID) (Run, error) {
	var run Run
	err := r.db.Preload("UserMessage").Preload("AssistantMessage").Where("id = ?", runID).First(&run).Error
	return run, err
}

func (r *Repository) ClaimNextQueuedRun() (Run, error) {
	var run Run
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ?", "queued").Order("created_at ASC").First(&run).Error; err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&Run{}).Where("id = ? AND status = ?", run.ID, "queued").Updates(map[string]any{"status": "running", "started_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Preload("UserMessage").First(&run, "id = ?", run.ID).Error
	})
	return run, err
}

func (r *Repository) UpdateRun(tx *gorm.DB, runID uuid.UUID, values map[string]any) error {
	if tx == nil {
		tx = r.db
	}
	values["updated_at"] = time.Now()
	return tx.Model(&Run{}).Where("id = ?", runID).Updates(values).Error
}

func (r *Repository) AttachAssistantMessage(tx *gorm.DB, runID uuid.UUID, assistantMessageID uuid.UUID, metadata datatypes.JSON) error {
	if tx == nil {
		tx = r.db
	}
	now := time.Now()
	return tx.Model(&Run{}).Where("id = ?", runID).Updates(map[string]any{
		"assistant_message_id": assistantMessageID,
		"metadata":             metadata,
		"status":               "completed",
		"completed_at":         now,
		"updated_at":           now,
	}).Error
}

func (r *Repository) TouchConversation(tx *gorm.DB, conversationID uuid.UUID) error {
	return tx.Model(&Conversation{}).Where("id = ?", conversationID).Update("updated_at", time.Now()).Error
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

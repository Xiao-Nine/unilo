package search

import (
	"encoding/json"

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

func (r *Repository) UpsertDocument(tx *gorm.DB, req IndexDocument) error {
	if tx == nil {
		tx = r.db
	}
	metadata := datatypes.JSON([]byte(`{}`))
	if req.Metadata != nil {
		raw, err := json.Marshal(req.Metadata)
		if err != nil {
			return err
		}
		metadata = datatypes.JSON(raw)
	}
	doc := Document{SourceType: req.SourceType, SourceID: req.SourceID, Title: req.Title, Content: req.Content, Metadata: metadata}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_type"}, {Name: "source_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"title":      req.Title,
			"content":    req.Content,
			"metadata":   metadata,
			"updated_at": gorm.Expr("now()"),
		}),
	}).Create(&doc).Error
}

func (r *Repository) DeleteDocument(tx *gorm.DB, sourceType string, sourceID string) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Where("source_type = ? AND source_id = ?", sourceType, sourceID).Delete(&Document{}).Error
}

func (r *Repository) Search(query string, sourceTypes []string, limit int) ([]Document, error) {
	return r.SearchWithOptions(query, sourceTypes, nil, limit)
}

func (r *Repository) SearchWithOptions(query string, sourceTypes []string, channelID *uuid.UUID, limit int) ([]Document, error) {
	like := "%" + query + "%"
	db := r.db.Where("(tsv @@ plainto_tsquery('simple', ?) OR content ILIKE ? OR COALESCE(title, '') ILIKE ?)", query, like, like)
	if len(sourceTypes) > 0 {
		db = db.Where("source_type IN ?", sourceTypes)
	}
	if channelID != nil {
		db = db.Where("metadata->>'channel_id' = ?", channelID.String())
	}
	var docs []Document
	err := db.Order(clause.Expr{SQL: "ts_rank(tsv, plainto_tsquery('simple', ?)) DESC, created_at DESC", Vars: []any{query}}).Limit(limit).Find(&docs).Error
	return docs, err
}

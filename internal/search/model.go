package search

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Document struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	SourceType string         `gorm:"size:30;not null;uniqueIndex:uq_search_documents_source"`
	SourceID   string         `gorm:"size:100;not null;uniqueIndex:uq_search_documents_source"`
	Title      string         `gorm:"type:text"`
	Content    string         `gorm:"type:text;not null"`
	TSV        string         `gorm:"type:tsvector"`
	Metadata   datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time      `gorm:"not null"`
	UpdatedAt  time.Time      `gorm:"not null"`
}

func (Document) TableName() string {
	return "search_documents"
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

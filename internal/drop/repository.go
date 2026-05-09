package drop

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

func (r *Repository) ListDrops(page int, size int) ([]Drop, int64, error) {
	var total int64
	if err := r.db.Model(&Drop{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var drops []Drop
	err := r.db.Preload("Author").Order("created_at DESC").Limit(size).Offset((page - 1) * size).Find(&drops).Error
	return drops, total, err
}

func (r *Repository) FindDrop(id uuid.UUID) (Drop, error) {
	var d Drop
	err := r.db.Preload("Author").First(&d, "id = ?", id).Error
	return d, err
}

func (r *Repository) CreateDrop(d *Drop) error {
	return r.db.Create(d).Error
}

func (r *Repository) DeleteDrop(d *Drop) error {
	return r.db.Delete(d).Error
}

func (r *Repository) ListComments(dropID uuid.UUID) ([]DropComment, error) {
	var comments []DropComment
	err := r.db.Preload("Author").Preload("ReplyToUser").Where("drop_id = ?", dropID).Order("created_at ASC").Find(&comments).Error
	return comments, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

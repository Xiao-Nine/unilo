package workspace

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

func (r *Repository) ListChildren(parentID *uuid.UUID) ([]File, error) {
	var files []File
	query := r.db.Order("is_folder DESC, name ASC, created_at ASC")
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	err := query.Find(&files).Error
	return files, err
}

func (r *Repository) ListChildrenAny(tx *gorm.DB, parentID uuid.UUID) ([]File, error) {
	var files []File
	err := tx.Unscoped().Where("parent_id = ?", parentID).Order("is_folder DESC, name ASC, created_at ASC").Find(&files).Error
	return files, err
}

func (r *Repository) ListTrash() ([]File, error) {
	var files []File
	err := r.db.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC, updated_at DESC").Find(&files).Error
	return files, err
}

func (r *Repository) FindActive(id uuid.UUID) (File, error) {
	var f File
	err := r.db.First(&f, "id = ?", id).Error
	return f, err
}

func (r *Repository) FindAny(id uuid.UUID) (File, error) {
	var f File
	err := r.db.Unscoped().First(&f, "id = ?", id).Error
	return f, err
}

func (r *Repository) FindDeleted(id uuid.UUID) (File, error) {
	var f File
	err := r.db.Unscoped().Where("id = ? AND deleted_at IS NOT NULL", id).First(&f).Error
	return f, err
}

func (r *Repository) Create(file *File) error {
	return r.db.Create(file).Error
}

func (r *Repository) Save(file *File) error {
	return r.db.Save(file).Error
}

func (r *Repository) FindByHash(fileHash string) (File, error) {
	var f File
	err := r.db.Where("is_folder = false AND file_hash = ?", fileHash).Order("created_at ASC").First(&f).Error
	return f, err
}

func (r *Repository) FindVersions(tx *gorm.DB, fileIDs []uuid.UUID) ([]FileVersion, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	var versions []FileVersion
	err := tx.Where("file_id IN ?", fileIDs).Find(&versions).Error
	return versions, err
}

func (r *Repository) CreateVersion(tx *gorm.DB, version *FileVersion) error {
	return tx.Create(version).Error
}

func (r *Repository) PermanentlyDelete(tx *gorm.DB, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Unscoped().Where("id IN ?", ids).Delete(&File{}).Error
}

func (r *Repository) Breadcrumbs(parentID *uuid.UUID) ([]File, error) {
	if parentID == nil {
		return nil, nil
	}
	var result []File
	currentID := parentID
	for currentID != nil {
		file, err := r.FindActive(*currentID)
		if err != nil {
			return nil, err
		}
		result = append([]File{file}, result...)
		currentID = file.ParentID
	}
	return result, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

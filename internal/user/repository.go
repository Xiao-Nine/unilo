package user

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

func (r *Repository) Create(u *User) error {
	return r.db.Create(u).Error
}

func (r *Repository) FindByUsername(username string) (User, error) {
	var u User
	err := r.db.Where("username = ?", username).First(&u).Error
	return u, err
}

func (r *Repository) FindByID(id uuid.UUID) (User, error) {
	var u User
	err := r.db.First(&u, "id = ?", id).Error
	return u, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

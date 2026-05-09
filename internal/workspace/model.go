package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"unilo/internal/user"
)

type File struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	ParentID    *uuid.UUID     `gorm:"type:uuid;index"`
	IsFolder    bool           `gorm:"not null"`
	Name        string         `gorm:"size:255;not null"`
	UploaderID  uuid.UUID      `gorm:"type:uuid;not null;index"`
	Uploader    user.User      `gorm:"foreignKey:UploaderID"`
	StorageType string         `gorm:"size:20"`
	ObjectKey   string         `gorm:"size:500"`
	SizeBytes   int64          `gorm:"not null;default:0"`
	MimeType    string         `gorm:"size:100"`
	FileHash    string         `gorm:"size:128;index"`
	Metadata    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	DeletedBy   *uuid.UUID     `gorm:"type:uuid"`
}

type FileVersion struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	FileID    uuid.UUID `gorm:"type:uuid;not null;index"`
	File      File      `gorm:"foreignKey:FileID"`
	EditorID  uuid.UUID `gorm:"type:uuid;not null;index"`
	Editor    user.User `gorm:"foreignKey:EditorID"`
	ObjectKey string    `gorm:"size:500;not null"`
	FileHash  string    `gorm:"size:128;not null"`
	SizeBytes int64     `gorm:"not null"`
	MimeType  string    `gorm:"size:100"`
	CreatedAt time.Time `gorm:"not null"`
}

func (File) TableName() string {
	return "workspace_files"
}

func (FileVersion) TableName() string {
	return "workspace_file_versions"
}

func (f *File) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

func (v *FileVersion) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

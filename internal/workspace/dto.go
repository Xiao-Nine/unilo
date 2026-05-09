package workspace

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Breadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FileResponse struct {
	ID          uuid.UUID       `json:"id"`
	ParentID    *uuid.UUID      `json:"parent_id"`
	IsFolder    bool            `json:"is_folder"`
	Name        string          `json:"name"`
	UploaderID  uuid.UUID       `json:"uploader_id"`
	SizeBytes   int64           `json:"size_bytes"`
	MimeType    string          `json:"mime_type"`
	FileHash    string          `json:"file_hash"`
	Metadata    json.RawMessage `json:"metadata"`
	PreviewURL  string          `json:"preview_url"`
	DownloadURL string          `json:"download_url"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type FileListResponse struct {
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs"`
	Files       []FileResponse `json:"files"`
}

type CreateFolderRequest struct {
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id"`
}

type CheckFileRequest struct {
	FileHash string     `json:"file_hash"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id"`
}

type CheckFileResponse struct {
	Exists bool          `json:"exists"`
	FileID *uuid.UUID    `json:"file_id"`
	File   *FileResponse `json:"file,omitempty"`
}

type RenameFileRequest struct {
	Name string `json:"name"`
}

type MoveFileRequest struct {
	TargetParentID *uuid.UUID `json:"target_parent_id"`
}

type RestoreFileRequest struct {
	TargetParentID *uuid.UUID `json:"target_parent_id"`
}

type TrashListResponse struct {
	Files []FileResponse `json:"files"`
}

type SaveContentRequest struct {
	Content  string `json:"content"`
	FileHash string `json:"file_hash"`
}

type DeleteFileResponse struct {
	Deleted bool      `json:"deleted"`
	TrashID uuid.UUID `json:"trash_id"`
}

type PurgeFileResponse struct {
	Purged bool `json:"purged"`
}

func ToFileResponse(f File) FileResponse {
	metadata := json.RawMessage(f.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	resp := FileResponse{
		ID:         f.ID,
		ParentID:   f.ParentID,
		IsFolder:   f.IsFolder,
		Name:       f.Name,
		UploaderID: f.UploaderID,
		SizeBytes:  f.SizeBytes,
		MimeType:   f.MimeType,
		FileHash:   f.FileHash,
		Metadata:   metadata,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
	if !f.IsFolder {
		resp.PreviewURL = "/api/v1/workspace/files/" + f.ID.String() + "/preview"
		resp.DownloadURL = "/api/v1/workspace/files/" + f.ID.String() + "/download"
	}
	return resp
}

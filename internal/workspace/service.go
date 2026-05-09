package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"unilo/internal/storage"
	"unilo/pkg/apperror"
)

type Indexer interface {
	UpsertDocument(tx *gorm.DB, sourceType string, sourceID string, title string, content string, metadata map[string]any) error
	DeleteDocument(tx *gorm.DB, sourceType string, sourceID string) error
}

type noopIndexer struct{}

func (noopIndexer) UpsertDocument(*gorm.DB, string, string, string, string, map[string]any) error {
	return nil
}

func (noopIndexer) DeleteDocument(*gorm.DB, string, string) error {
	return nil
}

type Service struct {
	repo           *Repository
	storage        storage.Client
	indexer        Indexer
	maxUploadBytes int64
}

type UploadInput struct {
	ParentID *uuid.UUID
	FileHash string
	Header   *multipart.FileHeader
}

type StreamFile struct {
	File        File
	Reader      io.ReadCloser
	Info        storage.ObjectInfo
	Disposition string
}

func NewService(repo *Repository, storageClient storage.Client, indexer Indexer, maxUploadBytes int64) *Service {
	if indexer == nil {
		indexer = noopIndexer{}
	}
	return &Service{repo: repo, storage: storageClient, indexer: indexer, maxUploadBytes: maxUploadBytes}
}

func (s *Service) ListFiles(parentID *uuid.UUID) (FileListResponse, error) {
	if err := s.validateParent(parentID); err != nil {
		return FileListResponse{}, err
	}
	breadcrumbs, err := s.breadcrumbs(parentID)
	if err != nil {
		return FileListResponse{}, err
	}
	files, err := s.repo.ListChildren(parentID)
	if err != nil {
		return FileListResponse{}, apperror.Internal(err)
	}
	respFiles := make([]FileResponse, 0, len(files))
	for _, f := range files {
		respFiles = append(respFiles, ToFileResponse(f))
	}
	return FileListResponse{Breadcrumbs: breadcrumbs, Files: respFiles}, nil
}

func (s *Service) CreateFolder(userID uuid.UUID, req CreateFolderRequest) (FileResponse, error) {
	name, err := validateName(req.Name)
	if err != nil {
		return FileResponse{}, err
	}
	if err := s.validateParent(req.ParentID); err != nil {
		return FileResponse{}, err
	}
	folder := File{ParentID: req.ParentID, IsFolder: true, Name: name, UploaderID: userID, Metadata: datatypes.JSON([]byte(`{}`))}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&folder).Error; err != nil {
			return mapDBError(err)
		}
		return s.indexFile(tx, folder)
	}); err != nil {
		return FileResponse{}, err
	}
	return ToFileResponse(folder), nil
}

func (s *Service) CheckFile(req CheckFileRequest) (CheckFileResponse, error) {
	if strings.TrimSpace(req.FileHash) == "" {
		return CheckFileResponse{}, apperror.BadRequest("file_hash is required")
	}
	if err := s.validateParent(req.ParentID); err != nil {
		return CheckFileResponse{}, err
	}
	file, err := s.repo.FindByHash(strings.TrimSpace(req.FileHash))
	if err != nil {
		if IsNotFound(err) {
			return CheckFileResponse{Exists: false}, nil
		}
		return CheckFileResponse{}, apperror.Internal(err)
	}
	resp := ToFileResponse(file)
	return CheckFileResponse{Exists: true, FileID: &file.ID, File: &resp}, nil
}

func (s *Service) Upload(ctx context.Context, userID uuid.UUID, input UploadInput) (FileResponse, error) {
	if input.Header == nil {
		return FileResponse{}, apperror.BadRequest("file is required")
	}
	fileHash := strings.ToLower(strings.TrimSpace(input.FileHash))
	if fileHash == "" {
		return FileResponse{}, apperror.BadRequest("file_hash is required")
	}
	if input.Header.Size <= 0 {
		return FileResponse{}, apperror.BadRequest("file is empty")
	}
	if s.maxUploadBytes > 0 && input.Header.Size > s.maxUploadBytes {
		return FileResponse{}, apperror.BadRequest("file is too large")
	}
	name, err := validateName(input.Header.Filename)
	if err != nil {
		return FileResponse{}, err
	}
	if err := s.validateParent(input.ParentID); err != nil {
		return FileResponse{}, err
	}

	fileID := uuid.New()
	objectKey := objectKey(fileID, fileHash, name)
	src, err := input.Header.Open()
	if err != nil {
		return FileResponse{}, apperror.Internal(err)
	}
	defer src.Close()

	var sniff bytes.Buffer
	hash := sha256.New()
	limited := io.LimitReader(src, input.Header.Size)
	reader := io.TeeReader(limited, hash)
	tee := io.TeeReader(reader, &sniff)
	contentType := input.Header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = detectContentType(input.Header.Filename, sniffReader{Reader: tee, sniff: &sniff})
	} else if _, _, err := mime.ParseMediaType(contentType); err != nil {
		contentType = "application/octet-stream"
	}

	body := io.MultiReader(bytes.NewReader(sniff.Bytes()), reader)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.storage.Put(ctx, objectKey, body, input.Header.Size, contentType); err != nil {
		return FileResponse{}, apperror.Internal(err)
	}
	computedHash := hex.EncodeToString(hash.Sum(nil))
	if computedHash != fileHash {
		_ = s.storage.Delete(ctx, objectKey)
		return FileResponse{}, apperror.BadRequest("file_hash does not match uploaded content")
	}

	metadata, _ := json.Marshal(map[string]any{"original_name": name})
	f := File{ID: fileID, ParentID: input.ParentID, IsFolder: false, Name: name, UploaderID: userID, StorageType: "s3", ObjectKey: objectKey, SizeBytes: input.Header.Size, MimeType: contentType, FileHash: fileHash, Metadata: datatypes.JSON(metadata)}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&f).Error; err != nil {
			return mapDBError(err)
		}
		return s.indexFile(tx, f)
	}); err != nil {
		_ = s.storage.Delete(ctx, objectKey)
		return FileResponse{}, err
	}
	return ToFileResponse(f), nil
}

func (s *Service) Download(ctx context.Context, fileID uuid.UUID) (StreamFile, error) {
	return s.stream(ctx, fileID, "attachment")
}

func (s *Service) Preview(ctx context.Context, fileID uuid.UUID) (StreamFile, error) {
	stream, err := s.stream(ctx, fileID, "inline")
	if err != nil {
		return StreamFile{}, err
	}
	if !previewable(stream.File.MimeType) {
		_ = stream.Reader.Close()
		return StreamFile{}, apperror.BadRequest("preview is unsupported")
	}
	return stream, nil
}

func (s *Service) Rename(userID uuid.UUID, fileID uuid.UUID, req RenameFileRequest) (FileResponse, error) {
	name, err := validateName(req.Name)
	if err != nil {
		return FileResponse{}, err
	}
	f, err := s.repo.FindActive(fileID)
	if err != nil {
		if IsNotFound(err) {
			return FileResponse{}, apperror.NotFound("file is not found")
		}
		return FileResponse{}, apperror.Internal(err)
	}
	f.Name = name
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&f).Error; err != nil {
			return mapDBError(err)
		}
		return s.indexFile(tx, f)
	}); err != nil {
		return FileResponse{}, err
	}
	return ToFileResponse(f), nil
}

func (s *Service) Move(userID uuid.UUID, fileID uuid.UUID, req MoveFileRequest) (FileResponse, error) {
	f, err := s.repo.FindActive(fileID)
	if err != nil {
		if IsNotFound(err) {
			return FileResponse{}, apperror.NotFound("file is not found")
		}
		return FileResponse{}, apperror.Internal(err)
	}
	if req.TargetParentID != nil {
		if *req.TargetParentID == f.ID {
			return FileResponse{}, apperror.BadRequest("folder cannot be moved into itself")
		}
		if err := s.validateParent(req.TargetParentID); err != nil {
			return FileResponse{}, err
		}
		if f.IsFolder {
			descendantIDs, err := collectDescendantIDs(s.repo.DB(), f.ID)
			if err != nil {
				return FileResponse{}, apperror.Internal(err)
			}
			if containsUUID(descendantIDs, *req.TargetParentID) {
				return FileResponse{}, apperror.BadRequest("folder cannot be moved into its descendant")
			}
		}
	}
	f.ParentID = req.TargetParentID
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&f).Error; err != nil {
			return mapDBError(err)
		}
		return s.indexFile(tx, f)
	}); err != nil {
		return FileResponse{}, err
	}
	return ToFileResponse(f), nil
}

func (s *Service) Delete(userID uuid.UUID, fileID uuid.UUID) (DeleteFileResponse, error) {
	file, err := s.repo.FindActive(fileID)
	if err != nil {
		if IsNotFound(err) {
			return DeleteFileResponse{}, apperror.NotFound("file is not found")
		}
		return DeleteFileResponse{}, apperror.Internal(err)
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		ids, err := collectDescendantIDs(tx, file.ID)
		if err != nil {
			return err
		}
		ids = append(ids, file.ID)
		now := time.Now()
		if err := tx.Model(&File{}).Where("id IN ?", ids).Updates(map[string]any{"deleted_at": now, "deleted_by": userID}).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.indexer.DeleteDocument(tx, "file", id.String()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return DeleteFileResponse{}, apperror.Internal(err)
	}
	return DeleteFileResponse{Deleted: true, TrashID: file.ID}, nil
}

func (s *Service) ListTrash() (TrashListResponse, error) {
	files, err := s.repo.ListTrash()
	if err != nil {
		return TrashListResponse{}, apperror.Internal(err)
	}
	respFiles := make([]FileResponse, 0, len(files))
	for _, f := range files {
		respFiles = append(respFiles, ToFileResponse(f))
	}
	return TrashListResponse{Files: respFiles}, nil
}

func (s *Service) Restore(userID uuid.UUID, fileID uuid.UUID, req RestoreFileRequest) (FileResponse, error) {
	file, err := s.repo.FindDeleted(fileID)
	if err != nil {
		if IsNotFound(err) {
			return FileResponse{}, apperror.NotFound("file is not found in trash")
		}
		return FileResponse{}, apperror.Internal(err)
	}
	targetParentID, err := s.restoreParent(req.TargetParentID, file.ParentID)
	if err != nil {
		return FileResponse{}, err
	}
	if file.IsFolder && targetParentID != nil {
		descendants, err := collectDescendantFilesAny(s.repo.DB(), file.ID)
		if err != nil {
			return FileResponse{}, apperror.Internal(err)
		}
		for _, descendant := range descendants {
			if descendant.ID == *targetParentID {
				return FileResponse{}, apperror.BadRequest("folder cannot be restored into its descendant")
			}
		}
	}

	deletedFile := file
	file.ParentID = targetParentID
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		descendants, err := collectDescendantFilesAny(tx, file.ID)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Unscoped().Model(&File{}).Where("id = ?", file.ID).Updates(map[string]any{"parent_id": targetParentID, "deleted_at": nil, "deleted_by": nil, "updated_at": now}).Error; err != nil {
			return mapDBError(err)
		}
		file.DeletedAt = gorm.DeletedAt{}
		file.DeletedBy = nil
		file.UpdatedAt = now
		if err := s.indexFile(tx, file); err != nil {
			return err
		}
		for _, descendant := range descendants {
			if !sameDeletionBatch(deletedFile, descendant) {
				continue
			}
			if err := tx.Unscoped().Model(&File{}).Where("id = ?", descendant.ID).Updates(map[string]any{"deleted_at": nil, "deleted_by": nil, "updated_at": now}).Error; err != nil {
				return mapDBError(err)
			}
			descendant.DeletedAt = gorm.DeletedAt{}
			descendant.DeletedBy = nil
			descendant.UpdatedAt = now
			if err := s.indexFile(tx, descendant); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return FileResponse{}, err
	}
	return ToFileResponse(file), nil
}

func (s *Service) Purge(ctx context.Context, fileID uuid.UUID) (PurgeFileResponse, error) {
	file, err := s.repo.FindDeleted(fileID)
	if err != nil {
		if IsNotFound(err) {
			return PurgeFileResponse{}, apperror.NotFound("file is not found in trash")
		}
		return PurgeFileResponse{}, apperror.Internal(err)
	}
	descendants, err := collectDescendantFilesAny(s.repo.DB(), file.ID)
	if err != nil {
		return PurgeFileResponse{}, apperror.Internal(err)
	}
	files := append([]File{file}, descendants...)
	ids := make([]uuid.UUID, 0, len(files))
	objectKeys := map[string]struct{}{}
	for _, f := range files {
		ids = append(ids, f.ID)
		if !f.IsFolder && f.ObjectKey != "" {
			objectKeys[f.ObjectKey] = struct{}{}
		}
	}
	versions, err := s.repo.FindVersions(s.repo.DB(), ids)
	if err != nil {
		return PurgeFileResponse{}, apperror.Internal(err)
	}
	for _, version := range versions {
		if version.ObjectKey != "" {
			objectKeys[version.ObjectKey] = struct{}{}
		}
	}
	for key := range objectKeys {
		if err := s.storage.Delete(ctx, key); err != nil {
			return PurgeFileResponse{}, apperror.Internal(err)
		}
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if err := s.indexer.DeleteDocument(tx, "file", id.String()); err != nil {
				return err
			}
		}
		return s.repo.PermanentlyDelete(tx, ids)
	}); err != nil {
		return PurgeFileResponse{}, apperror.Internal(err)
	}
	return PurgeFileResponse{Purged: true}, nil
}

func (s *Service) SaveContent(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, req SaveContentRequest) (FileResponse, error) {
	f, err := s.repo.FindActive(fileID)
	if err != nil {
		if IsNotFound(err) {
			return FileResponse{}, apperror.NotFound("file is not found")
		}
		return FileResponse{}, apperror.Internal(err)
	}
	if f.IsFolder {
		return FileResponse{}, apperror.BadRequest("folder content cannot be saved")
	}
	contentType, ok := editableContentType(f.Name, f.MimeType)
	if !ok {
		return FileResponse{}, apperror.BadRequest("file content is not editable")
	}
	content := []byte(req.Content)
	if s.maxUploadBytes > 0 && int64(len(content)) > s.maxUploadBytes {
		return FileResponse{}, apperror.BadRequest("content is too large")
	}
	checksum := sha256.Sum256(content)
	computedHash := hex.EncodeToString(checksum[:])
	requestedHash := strings.ToLower(strings.TrimSpace(req.FileHash))
	if requestedHash != "" && requestedHash != computedHash {
		return FileResponse{}, apperror.BadRequest("file_hash does not match content")
	}
	objectKey := objectKey(f.ID, computedHash, f.Name)
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(content), int64(len(content)), contentType); err != nil {
		return FileResponse{}, apperror.Internal(err)
	}

	f.StorageType = "s3"
	f.ObjectKey = objectKey
	f.SizeBytes = int64(len(content))
	f.MimeType = contentType
	f.FileHash = computedHash
	version := FileVersion{FileID: f.ID, EditorID: userID, ObjectKey: objectKey, FileHash: computedHash, SizeBytes: int64(len(content)), MimeType: contentType}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateVersion(tx, &version); err != nil {
			return err
		}
		if err := tx.Save(&f).Error; err != nil {
			return mapDBError(err)
		}
		return s.indexFileWithContent(tx, f, req.Content)
	}); err != nil {
		_ = s.storage.Delete(ctx, objectKey)
		return FileResponse{}, err
	}
	return ToFileResponse(f), nil
}

func (s *Service) stream(ctx context.Context, fileID uuid.UUID, disposition string) (StreamFile, error) {
	f, err := s.repo.FindActive(fileID)
	if err != nil {
		if IsNotFound(err) {
			return StreamFile{}, apperror.NotFound("file is not found")
		}
		return StreamFile{}, apperror.Internal(err)
	}
	if f.IsFolder {
		return StreamFile{}, apperror.BadRequest("folder cannot be downloaded")
	}
	reader, info, err := s.storage.Get(ctx, f.ObjectKey)
	if err != nil {
		return StreamFile{}, apperror.Internal(err)
	}
	if info.ContentType == "" {
		info.ContentType = f.MimeType
	}
	return StreamFile{File: f, Reader: reader, Info: info, Disposition: disposition}, nil
}

func (s *Service) validateParent(parentID *uuid.UUID) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.repo.FindActive(*parentID)
	if err != nil {
		if IsNotFound(err) {
			return apperror.NotFound("parent folder is not found")
		}
		return apperror.Internal(err)
	}
	if !parent.IsFolder {
		return apperror.BadRequest("parent_id is not a folder")
	}
	return nil
}

func (s *Service) restoreParent(requestedParentID *uuid.UUID, previousParentID *uuid.UUID) (*uuid.UUID, error) {
	if requestedParentID != nil {
		if err := s.validateParent(requestedParentID); err != nil {
			return nil, err
		}
		return requestedParentID, nil
	}
	if previousParentID == nil {
		return nil, nil
	}
	parent, err := s.repo.FindActive(*previousParentID)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, apperror.Internal(err)
	}
	if !parent.IsFolder {
		return nil, nil
	}
	return previousParentID, nil
}

func (s *Service) breadcrumbs(parentID *uuid.UUID) ([]Breadcrumb, error) {
	breadcrumbs := []Breadcrumb{{ID: "root", Name: "根目录"}}
	parents, err := s.repo.Breadcrumbs(parentID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperror.NotFound("parent folder is not found")
		}
		return nil, apperror.Internal(err)
	}
	for _, parent := range parents {
		breadcrumbs = append(breadcrumbs, Breadcrumb{ID: parent.ID.String(), Name: parent.Name})
	}
	return breadcrumbs, nil
}

func (s *Service) indexFile(tx *gorm.DB, f File) error {
	return s.indexFileWithContent(tx, f, f.Name)
}

func (s *Service) indexFileWithContent(tx *gorm.DB, f File, content string) error {
	return s.indexer.UpsertDocument(tx, "file", f.ID.String(), f.Name, content, map[string]any{
		"parent_id":   f.ParentID,
		"is_folder":   f.IsFolder,
		"mime_type":   f.MimeType,
		"size_bytes":  f.SizeBytes,
		"uploader_id": f.UploaderID,
	})
}

func validateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", apperror.BadRequest("name is required")
	}
	if len(name) > 255 {
		return "", apperror.BadRequest("name is too long")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return "", apperror.BadRequest("name is invalid")
	}
	return name, nil
}

func objectKey(fileID uuid.UUID, fileHash string, name string) string {
	now := time.Now()
	return fmt.Sprintf("workspace/%04d/%02d/%s/%s/%s", now.Year(), int(now.Month()), fileID.String(), fileHash, filepath.Base(name))
}

type sniffReader struct {
	io.Reader
	sniff *bytes.Buffer
}

func detectContentType(filename string, reader io.Reader) string {
	var buf bytes.Buffer
	_, _ = io.CopyN(&buf, reader, 512)
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}
	if buf.Len() == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf.Bytes())
}

func previewable(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "text/") || strings.HasPrefix(contentType, "image/png") || strings.HasPrefix(contentType, "image/jpeg") || strings.HasPrefix(contentType, "image/gif") || strings.HasPrefix(contentType, "image/webp") || contentType == "application/pdf"
}

func editableContentType(filename string, contentType string) (string, bool) {
	mediaType := strings.ToLower(strings.TrimSpace(contentType))
	if parsed, _, err := mime.ParseMediaType(mediaType); err == nil {
		mediaType = parsed
	}
	if strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml" || mediaType == "text/xml" || mediaType == "application/yaml" || mediaType == "application/x-yaml" {
		return contentType, true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown", ".txt", ".json", ".xml", ".yaml", ".yml", ".csv":
		byExt := mime.TypeByExtension(ext)
		if byExt == "" || byExt == "application/octet-stream" {
			byExt = "text/plain; charset=utf-8"
		}
		return byExt, true
	default:
		return contentType, false
	}
}

func collectDescendantIDs(tx *gorm.DB, parentID uuid.UUID) ([]uuid.UUID, error) {
	var children []File
	if err := tx.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
		if child.IsFolder {
			childIDs, err := collectDescendantIDs(tx, child.ID)
			if err != nil {
				return nil, err
			}
			ids = append(ids, childIDs...)
		}
	}
	return ids, nil
}

func collectDescendantFilesAny(tx *gorm.DB, parentID uuid.UUID) ([]File, error) {
	var children []File
	if err := tx.Unscoped().Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return nil, err
	}
	files := make([]File, 0, len(children))
	for _, child := range children {
		files = append(files, child)
		if child.IsFolder {
			childFiles, err := collectDescendantFilesAny(tx, child.ID)
			if err != nil {
				return nil, err
			}
			files = append(files, childFiles...)
		}
	}
	return files, nil
}

func sameDeletionBatch(parent File, child File) bool {
	if !parent.DeletedAt.Valid || !child.DeletedAt.Valid {
		return false
	}
	if (parent.DeletedBy == nil) != (child.DeletedBy == nil) {
		return false
	}
	if parent.DeletedBy != nil && *parent.DeletedBy != *child.DeletedBy {
		return false
	}
	return parent.DeletedAt.Time.Equal(child.DeletedAt.Time)
}

func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperror.Conflict("name already exists")
	}
	return apperror.Internal(err)
}

func ParseFileID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("file_id is invalid")
	}
	return id, nil
}

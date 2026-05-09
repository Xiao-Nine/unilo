package workspace

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"unilo/internal/auth"
	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListFiles(c *gin.Context) {
	parentID, err := optionalUUID(c.Query("parent_id"), "parent_id is invalid")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ListFiles(parentID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateFolder(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateFolder(userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CheckFile(c *gin.Context) {
	var req CheckFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CheckFile(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Upload(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	parentID, err := optionalUUID(c.PostForm("parent_id"), "parent_id is invalid")
	if err != nil {
		response.Error(c, err)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperror.BadRequest("file is required"))
		return
	}
	resp, err := h.service.Upload(c.Request.Context(), userID, UploadInput{ParentID: parentID, FileHash: c.PostForm("file_hash"), Header: header})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Download(c *gin.Context) {
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	stream, err := h.service.Download(c.Request.Context(), fileID)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer stream.Reader.Close()
	h.streamFile(c, stream)
}

func (h *Handler) Preview(c *gin.Context) {
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	stream, err := h.service.Preview(c.Request.Context(), fileID)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer stream.Reader.Close()
	h.streamFile(c, stream)
}

func (h *Handler) Rename(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req RenameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Rename(userID, fileID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Move(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req MoveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Move(userID, fileID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListTrash(c *gin.Context) {
	resp, err := h.service.ListTrash()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Restore(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req RestoreFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Restore(userID, fileID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Purge(c *gin.Context) {
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Purge(c.Request.Context(), fileID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SaveContent(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req SaveContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.SaveContent(c.Request.Context(), userID, fileID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Delete(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	fileID, err := ParseFileID(c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Delete(userID, fileID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) streamFile(c *gin.Context, stream StreamFile) {
	contentType := stream.Info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	if stream.Info.Size > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", stream.Info.Size))
	}
	disposition := mime.FormatMediaType(stream.Disposition, map[string]string{"filename": stream.File.Name})
	c.Header("Content-Disposition", disposition)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, stream.Reader)
}

func optionalUUID(raw string, message string) (*uuid.UUID, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	value, err := url.QueryUnescape(raw)
	if err != nil {
		return nil, apperror.BadRequest(message)
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return nil, apperror.BadRequest(message)
	}
	return &id, nil
}

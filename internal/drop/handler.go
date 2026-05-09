package drop

import (
	"strconv"

	"github.com/gin-gonic/gin"

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

func (h *Handler) ListDrops(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	page := 1
	if raw := c.Query("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.Error(c, apperror.BadRequest("page is invalid"))
			return
		}
		page = parsed
	}
	size := 20
	if raw := c.Query("size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.Error(c, apperror.BadRequest("size is invalid"))
			return
		}
		size = parsed
	}

	resp, err := h.service.ListDrops(userID, page, size)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateDrop(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	var req CreateDropRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateDrop(userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) GetDrop(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	dropID, err := ParseDropID(c.Param("drop_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.GetDrop(userID, dropID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeleteDrop(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	dropID, err := ParseDropID(c.Param("drop_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.DeleteDrop(userID, dropID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ToggleLike(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	dropID, err := ParseDropID(c.Param("drop_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ToggleLike(userID, dropID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateComment(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	dropID, err := ParseDropID(c.Param("drop_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateComment(userID, dropID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeleteComment(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	dropID, err := ParseDropID(c.Param("drop_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	commentID, err := ParseCommentID(c.Param("comment_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.DeleteComment(userID, dropID, commentID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

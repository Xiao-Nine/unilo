package agent

import (
	"strconv"

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

func (h *Handler) ListConversations(c *gin.Context) {
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
	resp, err := h.service.ListConversations(userID, page, size)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateConversation(userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListMessages(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	conversationID, err := parseConversationID(c.Param("conversation_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.Error(c, apperror.BadRequest("limit is invalid"))
			return
		}
		limit = parsed
	}
	resp, err := h.service.ListMessages(userID, conversationID, c.Query("cursor"), limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SendMessage(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	conversationID, err := parseConversationID(c.Param("conversation_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.SendMessage(c.Request.Context(), userID, conversationID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SubmitRun(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	conversationID, err := parseConversationID(c.Param("conversation_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.SubmitRun(c.Request.Context(), userID, conversationID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) GetRun(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	runID, err := parseRunID(c.Param("run_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.GetRun(userID, runID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func parseConversationID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("conversation_id is invalid")
	}
	return id, nil
}

func parseRunID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("run_id is invalid")
	}
	return id, nil
}

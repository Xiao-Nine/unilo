package channel

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

func (h *Handler) ListChannels(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	resp, err := h.service.ListChannels(userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateChannel(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateChannel(userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) UpdateChannel(c *gin.Context) {
	channelID, err := ParseChannelID(c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.UpdateChannel(channelID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeleteChannel(c *gin.Context) {
	channelID, err := ParseChannelID(c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.DeleteChannel(channelID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) MarkChannelRead(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	channelID, err := ParseChannelID(c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req MarkChannelReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.MarkChannelRead(userID, channelID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListMessages(c *gin.Context) {
	channelID, err := ParseChannelID(c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	cursor := int64(0)
	if raw := c.Query("cursor"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			response.Error(c, apperror.BadRequest("cursor is invalid"))
			return
		}
		cursor = parsed
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
	resp, err := h.service.ListMessages(channelID, cursor, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CreateMessage(c *gin.Context) {
	userID, ok := auth.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	channelID, err := ParseChannelID(c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.CreateMessage(channelID, userID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

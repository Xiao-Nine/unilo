package search

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Search(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.Error(c, apperror.BadRequest("limit is invalid"))
			return
		}
		limit = parsed
	}
	var channelID *uuid.UUID
	if raw := c.Query("channel_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil || parsed == uuid.Nil {
			response.Error(c, apperror.BadRequest("channel_id is invalid"))
			return
		}
		channelID = &parsed
	}
	resp, err := h.service.SearchWithOptions(SearchOptions{Query: c.Query("q"), SourceType: c.Query("type"), Limit: limit, ChannelID: channelID})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

package auth

import (
	"github.com/gin-gonic/gin"

	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Verify(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Register(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Login(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Refresh(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.BadRequest("invalid request body"))
		return
	}
	resp, err := h.service.Logout(req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Me(c *gin.Context) {
	userID, ok := CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.Unauthorized("unauthorized"))
		return
	}
	resp, err := h.service.Me(userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

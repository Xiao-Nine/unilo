package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"unilo/internal/auth"
	"unilo/pkg/apperror"
	"unilo/pkg/response"
)

type Handler struct {
	hub      *Hub
	tokens   *auth.TokenManager
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub, tokens *auth.TokenManager) *Handler {
	return &Handler{
		hub:    hub,
		tokens: tokens,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Handler) Handle(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		response.Error(c, apperror.Unauthorized("token is required"))
		return
	}
	claims, err := h.tokens.VerifyAccess(tokenString)
	if err != nil {
		response.Error(c, apperror.Unauthorized("access token is invalid"))
		return
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		response.Error(c, apperror.Unauthorized("access token is invalid"))
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{id: uuid.New(), hub: h.hub, conn: conn, send: make(chan []byte, 256), userID: userID}
	h.hub.register <- client
	go client.writeLoop()
	go client.readLoop()
}

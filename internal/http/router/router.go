package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"unilo/internal/agent"
	"unilo/internal/auth"
	"unilo/internal/channel"
	"unilo/internal/config"
	"unilo/internal/drop"
	"unilo/internal/http/middleware"
	"unilo/internal/realtime"
	"unilo/internal/search"
	"unilo/internal/workspace"
	"unilo/pkg/response"
)

type Dependencies struct {
	Config           config.Config
	Logger           *slog.Logger
	AuthHandler      *auth.Handler
	AuthTokens       *auth.TokenManager
	AgentHandler     *agent.Handler
	ChannelHandler   *channel.Handler
	DropHandler      *drop.Handler
	SearchHandler    *search.Handler
	WorkspaceHandler *workspace.Handler
	WSHandler        *realtime.Handler
}

func New(deps Dependencies) *gin.Engine {
	if deps.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.RequestLogger(deps.Logger))
	r.Use(middleware.CORS())

	r.GET("/healthz", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	api.GET("/healthz", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	authGroup := api.Group("/auth")
	authGroup.POST("/verify", deps.AuthHandler.Verify)
	authGroup.POST("/register", deps.AuthHandler.Register)
	authGroup.POST("/login", deps.AuthHandler.Login)
	authGroup.POST("/refresh", deps.AuthHandler.Refresh)
	authGroup.POST("/logout", deps.AuthHandler.Logout)
	authGroup.GET("/me", middleware.Auth(deps.AuthTokens), deps.AuthHandler.Me)

	api.GET("/ws", deps.WSHandler.Handle)

	protected := api.Group("")
	protected.Use(middleware.Auth(deps.AuthTokens))
	protected.GET("/agent/conversations", deps.AgentHandler.ListConversations)
	protected.POST("/agent/conversations", deps.AgentHandler.CreateConversation)
	protected.GET("/agent/conversations/:conversation_id/messages", deps.AgentHandler.ListMessages)
	protected.POST("/agent/conversations/:conversation_id/messages", deps.AgentHandler.SendMessage)
	protected.POST("/agent/conversations/:conversation_id/runs", deps.AgentHandler.SubmitRun)
	protected.GET("/agent/runs/:run_id", deps.AgentHandler.GetRun)
	protected.GET("/channels", deps.ChannelHandler.ListChannels)
	protected.POST("/channels", deps.ChannelHandler.CreateChannel)
	protected.PATCH("/channels/:channel_id", deps.ChannelHandler.UpdateChannel)
	protected.DELETE("/channels/:channel_id", deps.ChannelHandler.DeleteChannel)
	protected.POST("/channels/:channel_id/read", deps.ChannelHandler.MarkChannelRead)
	protected.GET("/channels/:channel_id/messages", deps.ChannelHandler.ListMessages)
	protected.POST("/channels/:channel_id/messages", deps.ChannelHandler.CreateMessage)

	protected.GET("/drops", deps.DropHandler.ListDrops)
	protected.POST("/drops", deps.DropHandler.CreateDrop)
	protected.GET("/drops/:drop_id", deps.DropHandler.GetDrop)
	protected.DELETE("/drops/:drop_id", deps.DropHandler.DeleteDrop)
	protected.POST("/drops/:drop_id/like", deps.DropHandler.ToggleLike)
	protected.POST("/drops/:drop_id/comments", deps.DropHandler.CreateComment)
	protected.DELETE("/drops/:drop_id/comments/:comment_id", deps.DropHandler.DeleteComment)

	protected.GET("/search", deps.SearchHandler.Search)

	protected.GET("/workspace/files", deps.WorkspaceHandler.ListFiles)
	protected.POST("/workspace/folders", deps.WorkspaceHandler.CreateFolder)
	protected.GET("/workspace/trash", deps.WorkspaceHandler.ListTrash)
	protected.POST("/workspace/files/check", deps.WorkspaceHandler.CheckFile)
	protected.POST("/workspace/files/upload", deps.WorkspaceHandler.Upload)
	protected.GET("/workspace/files/:file_id/download", deps.WorkspaceHandler.Download)
	protected.GET("/workspace/files/:file_id/preview", deps.WorkspaceHandler.Preview)
	protected.POST("/workspace/files/:file_id/move", deps.WorkspaceHandler.Move)
	protected.POST("/workspace/files/:file_id/restore", deps.WorkspaceHandler.Restore)
	protected.DELETE("/workspace/files/:file_id/purge", deps.WorkspaceHandler.Purge)
	protected.PUT("/workspace/files/:file_id/content", deps.WorkspaceHandler.SaveContent)
	protected.PATCH("/workspace/files/:file_id", deps.WorkspaceHandler.Rename)
	protected.DELETE("/workspace/files/:file_id", deps.WorkspaceHandler.Delete)

	return r
}

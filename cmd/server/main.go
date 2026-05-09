package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unilo/internal/agent"
	"unilo/internal/auth"
	"unilo/internal/channel"
	"unilo/internal/config"
	"unilo/internal/database"
	"unilo/internal/drop"
	"unilo/internal/http/router"
	"unilo/internal/realtime"
	"unilo/internal/search"
	"unilo/internal/storage"
	"unilo/internal/user"
	"unilo/internal/workspace"
	"unilo/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	logr := logger.New(cfg.App.Env)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	db, err := database.OpenPostgres(cfg.Database.URL)
	if err != nil {
		logr.Error("connect postgres failed", "error", err)
		os.Exit(1)
	}
	redisClient, err := database.OpenRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logr.Error("connect redis failed", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	tokens := auth.NewTokenManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	userRepo := user.NewRepository(db)
	authService := auth.NewService(db, userRepo, tokens, cfg)
	authHandler := auth.NewHandler(authService)

	searchRepo := search.NewRepository(db)
	searchService := search.NewService(searchRepo)
	searchHandler := search.NewHandler(searchService)

	agentRepo := agent.NewRepository(db)
	agentClient := agent.NewOpenAIClient(cfg.Agent)
	agentService := agent.NewService(agentRepo, searchService, agentClient, cfg.Agent)
	agentHandler := agent.NewHandler(agentService)

	storageClient, err := storage.NewMinIOClient(cfg.Storage)
	if err != nil {
		logr.Error("initialize storage failed", "error", err)
		os.Exit(1)
	}
	if err := storageClient.EnsureBucket(context.Background()); err != nil {
		logr.Error("ensure storage bucket failed", "error", err)
		os.Exit(1)
	}
	workspaceRepo := workspace.NewRepository(db)
	workspaceService := workspace.NewService(workspaceRepo, storageClient, searchService, cfg.Storage.MaxUploadBytes)
	workspaceHandler := workspace.NewHandler(workspaceService)

	realtimeBroker := realtime.NewRedisBroker(redisClient)
	presence := realtime.NewRedisPresence(redisClient)
	hub := realtime.NewHub(logr, realtimeBroker, presence)
	go hub.Run()
	go hub.Subscribe(appCtx)
	go agentService.StartWorker(appCtx)

	channelRepo := channel.NewRepository(db)
	channelService := channel.NewService(channelRepo, hub, searchService)
	hub.SetChannelService(channelService)
	hub.SetAgentService(agentService)
	agentService.SetNotifier(hub)
	channelHandler := channel.NewHandler(channelService)

	dropRepo := drop.NewRepository(db)
	dropService := drop.NewService(dropRepo, searchService)
	dropHandler := drop.NewHandler(dropService)

	wsHandler := realtime.NewHandler(hub, tokens)

	engine := router.New(router.Dependencies{
		Config:           cfg,
		Logger:           logr,
		AuthHandler:      authHandler,
		AuthTokens:       tokens,
		AgentHandler:     agentHandler,
		ChannelHandler:   channelHandler,
		DropHandler:      dropHandler,
		SearchHandler:    searchHandler,
		WorkspaceHandler: workspaceHandler,
		WSHandler:        wsHandler,
	})

	server := &http.Server{Addr: cfg.App.HTTPAddr, Handler: engine}
	go func() {
		logr.Info("server listening", "addr", cfg.App.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logr.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logr.Error("server shutdown failed", "error", err)
	}
}

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crosslogic/control-plane/internal/billing"
	"github.com/crosslogic/control-plane/internal/config"
	"github.com/crosslogic/control-plane/internal/gateway"
	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("starting CrossLogic Control Plane")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("connected to database")

	// Initialize Redis cache
	redisCache, err := cache.NewCache(cfg.Redis)
	if err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisCache.Close()
	logger.Info("connected to Redis")

	// Initialize billing engine
	billingEngine := billing.NewEngine(db, logger, cfg.Billing.StripeSecretKey)
	logger.Info("initialized billing engine")

	// Initialize webhook handler
	webhookHandler := billing.NewWebhookHandler(cfg.Billing.StripeWebhookSecret, db, logger)
	logger.Info("initialized webhook handler")

	// Initialize SkyPilot orchestrator
	orch, err := orchestrator.NewSkyPilotOrchestrator(db, logger, cfg.Server.ControlPlaneURL)
	if err != nil {
		logger.Fatal("failed to initialize orchestrator", zap.Error(err))
	}
	logger.Info("initialized SkyPilot orchestrator")

	// Start billing background jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	billingEngine.StartBackgroundJobs(ctx)

	// Initialize API gateway
	gw := gateway.NewGateway(db, redisCache, logger, webhookHandler, orch)
	logger.Info("initialized API gateway")

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      gw,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting HTTP server",
			zap.String("address", server.Addr),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

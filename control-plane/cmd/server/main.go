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
	"github.com/crosslogic/control-plane/internal/notifications"
	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
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

	// Initialize event bus
	eventBus := events.NewBus(logger)
	logger.Info("initialized event bus")

	// Initialize notification service
	notificationConfig, err := notifications.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load notification config", zap.Error(err))
	}

	notificationService, err := notifications.NewService(notificationConfig, db, redisCache, logger, eventBus)
	if err != nil {
		logger.Fatal("failed to initialize notification service", zap.Error(err))
	}
	logger.Info("initialized notification service")

	// Initialize billing engine
	billingEngine := billing.NewEngine(db, logger, cfg.Billing.StripeSecretKey)
	logger.Info("initialized billing engine")

	// Initialize webhook handler with event bus
	webhookHandler := billing.NewWebhookHandler(cfg.Billing.StripeWebhookSecret, db, redisCache, logger, eventBus)
	logger.Info("initialized webhook handler")

	// Initialize SkyPilot orchestrator with event bus
	orch, err := orchestrator.NewSkyPilotOrchestrator(
		db,
		logger,
		cfg.Server.ControlPlaneURL,
		cfg.Runtime.VLLMVersion,
		cfg.Runtime.TorchVersion,
		eventBus,
	)
	if err != nil {
		logger.Fatal("failed to initialize orchestrator", zap.Error(err))
	}
	logger.Info("initialized SkyPilot orchestrator")

	// Initialize Triple Safety Monitor
	monitor := orchestrator.NewTripleSafetyMonitor(db, logger, orch)
	logger.Info("initialized triple safety monitor")

	// Initialize State Reconciler
	reconciler := orchestrator.NewStateReconciler(db, logger, orch)
	logger.Info("initialized state reconciler")

	// Start background services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitor and reconciler
	monitor.Start(ctx)
	reconciler.Start(ctx)

	// Start billing background jobs
	billingEngine.StartBackgroundJobs(ctx)

	// Start notification service
	if err := notificationService.Start(ctx); err != nil {
		logger.Fatal("failed to start notification service", zap.Error(err))
	}
	logger.Info("started notification service")

	// Initialize API gateway with event bus
	gw := gateway.NewGateway(db, redisCache, logger, webhookHandler, orch, monitor, cfg.Security.AdminAPIToken, eventBus)
	gw.StartHealthMetrics(ctx)
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

	// Stop notification service
	if err := notificationService.Stop(shutdownCtx); err != nil {
		logger.Error("failed to stop notification service gracefully", zap.Error(err))
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

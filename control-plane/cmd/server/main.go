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
	"github.com/crosslogic/control-plane/internal/credentials"
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

	// Initialize billing engine when enabled
	var billingEngine *billing.Engine
	if cfg.Billing.Enabled {
		billingEngine = billing.NewEngine(db, logger, cfg.Billing.StripeSecretKey)
		logger.Info("initialized billing engine")
	} else {
		logger.Warn("billing disabled via configuration; skipping Stripe initialization")
	}

	// Initialize cost tracker for per-tenant cost aggregation
	costTracker := billing.NewCostTracker(db, logger)
	logger.Info("initialized cost tracker")

	// Initialize webhook handler with event bus when billing is enabled
	var webhookHandler *billing.WebhookHandler
	if cfg.Billing.Enabled {
		webhookHandler = billing.NewWebhookHandler(cfg.Billing.StripeWebhookSecret, db, redisCache, logger, eventBus)
		logger.Info("initialized webhook handler")
	} else {
		logger.Info("billing disabled; webhook handler not registered")
	}

	// Initialize SkyPilot orchestrator with event bus and API server config
	orch, err := orchestrator.NewSkyPilotOrchestrator(
		db,
		redisCache,
		logger,
		cfg.Server.ControlPlaneURL,
		cfg.Runtime.VLLMVersion,
		cfg.Runtime.TorchVersion,
		eventBus,
		cfg.R2,
		cfg.SkyPilot,
	)
	if err != nil {
		logger.Fatal("failed to initialize orchestrator", zap.Error(err))
	}
	logger.Info("initialized SkyPilot orchestrator")

	// Initialize Triple Safety Monitor
	monitor := orchestrator.NewTripleSafetyMonitor(db, logger, orch, eventBus)
	logger.Info("initialized triple safety monitor")

	// Initialize State Reconciler with Triple Safety Monitor integration
	reconciler := orchestrator.NewStateReconciler(db, logger, orch, monitor)
	logger.Info("initialized state reconciler")

	// Initialize credential service for cloud credential management
	var credentialService *credentials.Service
	if cfg.SkyPilot.UseAPIServer {
		if cfg.SkyPilot.CredentialEncryptionKey == "" {
			logger.Fatal("SKYPILOT_CREDENTIAL_ENCRYPTION_KEY is required when using API Server mode")
		}
		// Use a consistent key ID for encryption (in production, this should be versioned for key rotation)
		encryptionKeyID := "v1"
		credentialService, err = credentials.NewService(db, cfg.SkyPilot.CredentialEncryptionKey, encryptionKeyID, logger)
		if err != nil {
			logger.Fatal("failed to initialize credential service", zap.Error(err))
		}
		logger.Info("initialized credential service for cloud credential management")
	} else {
		logger.Info("credential service disabled (using CLI mode)")
	}

	// Start background services context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize API gateway with event bus and credential service
	gw := gateway.NewGateway(db, redisCache, logger, webhookHandler, orch, monitor, cfg.Security.AdminAPIToken, eventBus, credentialService)
	gw.StartHealthMetrics(ctx)

	// Start queue depth monitoring for intelligent load balancing
	gw.LoadBalancer.StartQueueMonitoring(ctx)
	logger.Info("initialized API gateway with queue monitoring")

	// Initialize Deployment Controller
	deploymentController := orchestrator.NewDeploymentController(db, logger, orch, gw.LoadBalancer)
	logger.Info("initialized deployment controller")

	// Initialize Model Cache Warmer for R2/vLLM optimization
	cacheWarmer := orchestrator.NewModelCacheWarmer(db, logger, orch)
	logger.Info("initialized model cache warmer")

	// Start monitor and reconciler
	monitor.Start(ctx)
	reconciler.Start(ctx)
	deploymentController.Start(ctx)

	// Start predictive cache warming
	cacheWarmer.Start(ctx)
	logger.Info("started predictive cache warming")

	// Start billing background jobs if billing is enabled
	if billingEngine != nil {
		billingEngine.StartBackgroundJobs(ctx)
	}

	// Start cost tracker aggregation loop (available even when billing disabled)
	costTracker.Start(ctx)
	logger.Info("started cost tracker")

	// Start notification service
	if err := notificationService.Start(ctx); err != nil {
		logger.Fatal("failed to start notification service", zap.Error(err))
	}
	logger.Info("started notification service")

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

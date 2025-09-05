package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	

	"github.com/xizzxy/helios/internal/config"
	"github.com/xizzxy/helios/internal/control"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Setup logger
	logger := setupLogger(cfg.Observability.LogLevel)
	logger.Info("Starting Helios Control Plane",
		"version", cfg.Observability.ServiceVersion,
		"address", cfg.Control.Address,
		"grpc_address", cfg.Control.GRPCAddress,
	)

	// Create control plane server
	server, err := control.NewServer(cfg, logger)
	if err != nil {
		logger.Error("Failed to create control server", "error", err)
		os.Exit(1)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		logger.Error("Failed to start control server", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Control.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("Control server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("Control plane shutdown complete")
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/yourorg/leaderboard/docs" // Import swagger docs
	pb "github.com/yourorg/leaderboard/gen/leaderboard/v1"
	"github.com/yourorg/leaderboard/internal/config"
	"github.com/yourorg/leaderboard/internal/log"
	"github.com/yourorg/leaderboard/internal/notify"
	"github.com/yourorg/leaderboard/internal/service"
	"github.com/yourorg/leaderboard/internal/store"
	grpcTransport "github.com/yourorg/leaderboard/internal/transport/grpc"
	restTransport "github.com/yourorg/leaderboard/internal/transport/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	logger := log.NewConsole(cfg.LogLevel)
	logger.Info().Msg("starting leaderboard server")

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection pool
	logger.Info().Msg("connecting to database")
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()
	logger.Info().Msg("database connection established")

	// Initialize store
	st := store.NewStore(pool)

	// Initialize notify listener
	listener := notify.NewListener(pool, logger.Logger)
	listener.Start(ctx)

	// Log listener errors in background
	go func() {
		for err := range listener.Errors() {
			logger.Error().Err(err).Msg("notify listener error")
		}
	}()

	// Initialize service layer
	svc := service.New(st, logger.Logger)

	// Initialize gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(1024*1024),     // 1MB
		grpc.MaxSendMsgSize(10*1024*1024),  // 10MB
		grpc.MaxConcurrentStreams(1000),
	)

	grpcHandler := grpcTransport.NewServer(svc, listener, logger.Logger, cfg.DefaultLimit, cfg.MaxLimit)
	pb.RegisterLeaderboardServiceServer(grpcServer, grpcHandler)

	// Enable gRPC reflection for grpcurl and similar tools
	reflection.Register(grpcServer)

	// Initialize REST server
	restServer := restTransport.NewServer(svc, logger.Logger)

	// Start gRPC server in goroutine
	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("create gRPC listener: %w", err)
	}

	grpcErrChan := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", grpcAddr).Msg("starting gRPC server")
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErrChan <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	// Start REST server in goroutine
	restAddr := fmt.Sprintf(":%s", cfg.RESTPort)
	restErrChan := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", restAddr).Msg("starting REST server")
		if err := restServer.Start(restAddr); err != nil {
			restErrChan <- fmt.Errorf("REST server: %w", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received or an error occurs
	select {
	case sig := <-sigChan:
		logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
	case err := <-grpcErrChan:
		return err
	case err := <-restErrChan:
		return err
	}

	// Graceful shutdown
	logger.Info().Msg("shutting down gracefully")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown REST server
	if err := restServer.Shutdown(); err != nil {
		logger.Error().Err(err).Msg("error shutting down REST server")
	}

	// Gracefully stop gRPC server
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-shutdownCtx.Done():
		logger.Warn().Msg("shutdown timeout exceeded, forcing stop")
		grpcServer.Stop()
	case <-stopped:
		logger.Info().Msg("gRPC server stopped gracefully")
	}

	// Cancel main context to stop notify listener
	cancel()

	logger.Info().Msg("shutdown complete")
	return nil
}

package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Bremcm/uptime/internal/config"
	billingv1 "github.com/Bremcm/uptime/internal/pb/billing/v1"
	"github.com/Bremcm/uptime/internal/storage"

	"google.golang.org/grpc"
)

type billingServer struct {
	billingv1.UnimplementedBillingServiceServer
	store *storage.Store
	log   *slog.Logger
}

func (s *billingServer) GetLimits(ctx context.Context, req *billingv1.GetLimitsRequest) (*billingv1.GetLimitsResponse, error) {
	sub, err := s.store.SubscriptionByUser(ctx, req.UserId)
	if err != nil {
		s.log.Error("failed to load subscription", "user", req.UserId, "error", err)
		return nil, err
	}

	return &billingv1.GetLimitsResponse{
		PlanName:           sub.Plan.Name,
		MaxMonitors:        int32(sub.Plan.MaxMonitors),
		MinIntervalSeconds: int32(sub.Plan.MinIntervalSeconds),
		RateLimitPerMinute: int32(sub.Plan.RateLimitPerMinute),
	}, nil
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	listener, err := net.Listen("tcp", cfg.BillingGRPCAddr)
	if err != nil {
		log.Error("failed to listen", "addr", cfg.BillingGRPCAddr, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	billingv1.RegisterBillingServiceServer(grpcServer, &billingServer{store: store, log: log})

	go func() {
		<-ctx.Done()
		log.Info("shutting down billing service")
		grpcServer.GracefulStop()
	}()

	log.Info("billing service started", "addr", cfg.BillingGRPCAddr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Error("grpc server stopped", "error", err)
	}
}

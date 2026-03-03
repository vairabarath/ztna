package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/igris/ztna/controller/internal/config"
	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/devicegrpc"
	"github.com/igris/ztna/controller/internal/enrollment"
	"github.com/igris/ztna/controller/internal/enrollmentgrpc"
	"github.com/igris/ztna/controller/internal/grpcserver"
	"github.com/igris/ztna/controller/internal/revocation"
	"github.com/igris/ztna/controller/internal/storage"
	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
	"github.com/igris/ztna/controller/internal/workspacegrpc"
	controlplanev1 "github.com/igris/ztna/proto/gen/go/ztna/controlplane/v1"
)

func main() {
	cfg := config.FromEnv()

	db, err := storage.OpenPostgres(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer startupCancel()

	if err := storage.Ping(startupCtx, db); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}
	if err := storage.RunMigrations(startupCtx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	srv, err := grpcserver.New(cfg)
	if err != nil {
		log.Fatalf("init gRPC server: %v", err)
	}

	encryptor, err := workspace.NewAESGCMEncryptorFromBase64(cfg.CAEncKeyB64)
	if err != nil {
		log.Fatalf("init workspace key encryptor: %v", err)
	}

	workspaceRepo := workspace.NewPostgresRepository(db)
	tokenRepo := token.NewPostgresRepository(db)
	deviceRepo := device.NewPostgresRepository(db)
	revocationRepo := revocation.NewPostgresRepository(db)

	workspaceSvc := workspace.NewService(workspaceRepo, encryptor)
	tokenSvc := token.NewService(tokenRepo)
	revocationSvc := revocation.NewService(revocationRepo, revocation.NewCache(), revocation.NewBroker())
	enrollmentSvc := enrollment.NewService(
		workspaceRepo,
		tokenSvc,
		deviceRepo,
		encryptor,
		enrollment.NewLocalSigner(),
		storage.NewPostgresTxRunner(db),
	)

	workspaceHandler := workspacegrpc.NewHandler(workspaceSvc, tokenSvc)
	enrollmentHandler := enrollmentgrpc.NewHandler(
		enrollmentSvc,
		workspaceSvc,
		deviceRepo,
		revocationSvc,
		cfg.TLSEnabled,
	)
	deviceHandler := devicegrpc.NewHandler(deviceRepo, revocationSvc)
	controlplanev1.RegisterWorkspaceServiceServer(srv.GRPC(), workspaceHandler)
	controlplanev1.RegisterEnrollmentServiceServer(srv.GRPC(), enrollmentHandler)
	controlplanev1.RegisterDeviceServiceServer(srv.GRPC(), deviceHandler)

	go func() {
		log.Printf("controller gRPC listening on %s", cfg.ListenAddr)
		if err := srv.Start(); err != nil {
			log.Fatalf("serve gRPC: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.GracefulStop(shutdownCtx); err != nil {
		log.Printf("graceful stop timeout, forcing stop: %v", err)
		srv.Stop()
	}
}

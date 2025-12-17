//go:build grpcserver

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"droneDeliveryManagement/internal/config"
	"droneDeliveryManagement/internal/db"
	grpcserver "droneDeliveryManagement/internal/grpc"
	"droneDeliveryManagement/repository"
)

func main() {
	// Load configuration
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("Configuration loaded: %v", cfg)

	// Open DB
	d, err := db.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)

	// Start gRPC
	shutdown, err := grpcserver.StartGRPC(cfg, users, orders, drones)
	if err != nil {
		log.Fatalf("start grpc: %v", err)
	}
	log.Printf("gRPC server listening on %s", cfg.GRPC.Address)

	// Wait for signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

//go:build grpcserver

package grpcserver

import (
	"context"
	"net"

	adminv1 "droneDeliveryManagement/api/admin/v1"
	dronev1 "droneDeliveryManagement/api/drone/v1"
	userv1 "droneDeliveryManagement/api/user/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/internal/config"
	"droneDeliveryManagement/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const healthCheckMethod = "/grpc.health.v1.Health/Check"

// StartGRPC starts the gRPC server on the given address and returns a shutdown function.
// The server implements UserOrderService, DroneService, and AdminService with authentication interceptor.
func StartGRPC(cfg *config.Config, users *repository.UserRepository, orders *repository.OrderRepository, drones *repository.DroneRepository) (func(context.Context) error, error) {
	if cfg == nil {
		panic("config is required")
	}

	addr := cfg.GRPC.Address
	if addr == "" {
		addr = ":50051"
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Allow plaintext for simplicity; in production, configure TLS.
	_ = insecure.NewCredentials

	srv := grpc.NewServer(grpc.UnaryInterceptor(auth.NewUnaryAuthInterceptor(cfg.Auth.JWTSecret, healthCheckMethod)))

	// Register User Order Service.
	s := &Server{Users: users, Orders: orders, Drones: drones}
	userv1.RegisterUserOrderServiceServer(srv, s)

	// Register Drone Service.
	ds := &DroneServer{Users: users, Orders: orders, Drones: drones}
	dronev1.RegisterDroneServiceServer(srv, ds)

	// Register Admin Service.
	as := &AdminServer{Users: users, Orders: orders, Drones: drones}
	adminv1.RegisterAdminServiceServer(srv, as)

	go func() { _ = srv.Serve(lis) }()

	return func(ctx context.Context) error {
		done := make(chan struct{})
		go func() { srv.GracefulStop(); close(done) }()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			srv.Stop()
			return ctx.Err()
		}
	}, nil
}

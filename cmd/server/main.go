package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	"github.com/sartim/storemesh-inventory-service/internal/auth"
	"github.com/sartim/storemesh-inventory-service/internal/observability"
	"github.com/sartim/storemesh-inventory-service/internal/repository"
	"github.com/sartim/storemesh-inventory-service/internal/service"
	"google.golang.org/grpc"
)

func main() {
	grpcAddr := env("GRPC_ADDR", ":50051")
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal(err)
	}
	oidc, err := auth.NewValidator(os.Getenv("KEYCLOAK_ISSUER"), os.Getenv("KEYCLOAK_AUDIENCE"))
	if err != nil {
		log.Fatalf("configure Keycloak OIDC: %v", err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(auth.UnaryInterceptor(oidc)))
	go serveMetrics(env("METRICS_ADDR", ":8080"))
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := repository.Open(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		inventoryv1.RegisterInventoryServiceServer(server, service.NewPersistentInventory(store))
	} else {
		inventoryv1.RegisterInventoryServiceServer(server, service.NewInventory())
	}
	log.Println("inventory service listening on " + grpcAddr)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.Handler())
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Println("inventory metrics listening on " + addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("metrics server: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

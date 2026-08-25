package main

import (
	"log"
	"net"

	inventoryv1 "storemesh-inventory-service/gen/storemesh/inventory/v1"
	"storemesh-inventory-service/internal/service"
	"google.golang.org/grpc"
)

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil { log.Fatal(err) }
	server := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(server, service.NewInventory())
	log.Println("inventory service listening on :50051")
	if err := server.Serve(listener); err != nil { log.Fatal(err) }
}

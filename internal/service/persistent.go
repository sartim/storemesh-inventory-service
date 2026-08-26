package service

import (
	"context"
	"database/sql"

	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	"github.com/sartim/storemesh-inventory-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PersistentInventory struct {
	inventoryv1.UnimplementedInventoryServiceServer
	store *repository.Store
}

func NewPersistentInventory(store *repository.Store) *PersistentInventory {
	return &PersistentInventory{store: store}
}

func (i *PersistentInventory) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	stock, err := i.store.Get(ctx, req.GetProductId())
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "stock not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock: %v", err)
	}
	return &inventoryv1.GetStockResponse{Stock: stock}, nil
}

func (i *PersistentInventory) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetAdjustment() == nil {
		return nil, status.Error(codes.InvalidArgument, "product ID and adjustment are required")
	}
	if err := i.store.Adjust(ctx, req.GetProductId(), req.GetAdjustment().GetQuantityDelta()); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "adjust stock: %v", err)
	}
	stock, err := i.store.Get(ctx, req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock: %v", err)
	}
	return &inventoryv1.AdjustStockResponse{Stock: stock}, nil
}

func (i *PersistentInventory) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetReservation() == nil || req.GetReservation().GetQuantity() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product ID and positive reservation quantity are required")
	}
	if err := i.store.Reserve(ctx, req.GetProductId(), req.GetReservation().GetReservationId(), req.GetReservation().GetQuantity()); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "reserve stock: %v", err)
	}
	stock, err := i.store.Get(ctx, req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock: %v", err)
	}
	return &inventoryv1.ReserveStockResponse{Stock: stock}, nil
}

func (i *PersistentInventory) ReleaseReservation(ctx context.Context, req *inventoryv1.ReleaseReservationRequest) (*inventoryv1.ReleaseReservationResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetReservation() == nil || req.GetReservation().GetReservationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "product ID and reservation ID are required")
	}
	if err := i.store.Release(ctx, req.GetProductId(), req.GetReservation().GetReservationId()); err != nil {
		return nil, status.Errorf(codes.NotFound, "release reservation: %v", err)
	}
	stock, err := i.store.Get(ctx, req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stock: %v", err)
	}
	return &inventoryv1.ReleaseReservationResponse{Stock: stock}, nil
}

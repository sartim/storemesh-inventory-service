package service

import (
	"context"
	"sync"

	"github.com/google/uuid"
	inventoryv1 "storemesh-inventory-service/gen/storemesh/inventory/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Inventory struct {
	inventoryv1.UnimplementedInventoryServiceServer
	mu           sync.RWMutex
	stock        map[string]*inventoryv1.Stock
	reservations map[string]map[string]int64
}

func NewInventory() *Inventory {
	return &Inventory{stock: make(map[string]*inventoryv1.Stock), reservations: make(map[string]map[string]int64)}
}

func (i *Inventory) GetStock(_ context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	stock, ok := i.stock[req.GetProductId()]
	if !ok { return nil, status.Error(codes.NotFound, "stock not found") }
	return &inventoryv1.GetStockResponse{Stock: clone(stock)}, nil
}

func (i *Inventory) AdjustStock(_ context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetAdjustment() == nil { return nil, status.Error(codes.InvalidArgument, "product ID and adjustment are required") }
	i.mu.Lock()
	defer i.mu.Unlock()
	stock := i.ensure(req.GetProductId())
	if stock.OnHand+req.GetAdjustment().GetQuantityDelta() < stock.Reserved { return nil, status.Error(codes.FailedPrecondition, "adjustment would reduce stock below reserved quantity") }
	stock.OnHand += req.GetAdjustment().GetQuantityDelta()
	stock.Available = stock.OnHand - stock.Reserved
	stock.UpdatedAt = timestamppb.Now()
	return &inventoryv1.AdjustStockResponse{Stock: clone(stock)}, nil
}

func (i *Inventory) ReserveStock(_ context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetReservation() == nil || req.GetReservation().GetQuantity() <= 0 { return nil, status.Error(codes.InvalidArgument, "product ID and positive reservation quantity are required") }
	i.mu.Lock()
	defer i.mu.Unlock()
	stock := i.ensure(req.GetProductId())
	if stock.Available < req.GetReservation().GetQuantity() { return nil, status.Error(codes.FailedPrecondition, "insufficient available stock") }
	reservationID := req.GetReservation().GetReservationId()
	if reservationID == "" { reservationID = uuid.NewString() }
	if _, exists := i.reservations[req.GetProductId()][reservationID]; exists { return nil, status.Error(codes.AlreadyExists, "reservation already exists") }
	i.reservations[req.GetProductId()][reservationID] = req.GetReservation().GetQuantity()
	stock.Reserved += req.GetReservation().GetQuantity()
	stock.Available = stock.OnHand - stock.Reserved
	stock.UpdatedAt = timestamppb.Now()
	return &inventoryv1.ReserveStockResponse{Stock: clone(stock)}, nil
}

func (i *Inventory) ReleaseReservation(_ context.Context, req *inventoryv1.ReleaseReservationRequest) (*inventoryv1.ReleaseReservationResponse, error) {
	if req == nil || req.GetProductId() == "" || req.GetReservation() == nil || req.GetReservation().GetReservationId() == "" { return nil, status.Error(codes.InvalidArgument, "product ID and reservation ID are required") }
	i.mu.Lock()
	defer i.mu.Unlock()
	stock := i.ensure(req.GetProductId())
	quantity, ok := i.reservations[req.GetProductId()][req.GetReservation().GetReservationId()]
	if !ok { return nil, status.Error(codes.NotFound, "reservation not found") }
	delete(i.reservations[req.GetProductId()], req.GetReservation().GetReservationId())
	stock.Reserved -= quantity
	stock.Available = stock.OnHand - stock.Reserved
	stock.UpdatedAt = timestamppb.Now()
	return &inventoryv1.ReleaseReservationResponse{Stock: clone(stock)}, nil
}

func (i *Inventory) ensure(productID string) *inventoryv1.Stock {
	if stock, ok := i.stock[productID]; ok { return stock }
	stock := &inventoryv1.Stock{ProductId: productID, UpdatedAt: timestamppb.Now()}
	i.stock[productID] = stock
	i.reservations[productID] = make(map[string]int64)
	return stock
}

func clone(stock *inventoryv1.Stock) *inventoryv1.Stock { copy := *stock; return &copy }

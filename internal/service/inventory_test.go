package service

import (
	"context"
	"testing"

	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInventoryReservationLifecycle(t *testing.T) {
	i := NewInventory()
	ctx := context.Background()
	_, err := i.AdjustStock(ctx, &inventoryv1.AdjustStockRequest{ProductId: "p-1", Adjustment: &inventoryv1.StockAdjustment{QuantityDelta: 5}})
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := i.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{ProductId: "p-1", Reservation: &inventoryv1.StockReservation{ReservationId: "r-1", Quantity: 2}})
	if err != nil || reserved.GetStock().GetAvailable() != 3 {
		t.Fatalf("reserve: stock=%v err=%v", reserved, err)
	}
	released, err := i.ReleaseReservation(ctx, &inventoryv1.ReleaseReservationRequest{ProductId: "p-1", Reservation: &inventoryv1.StockReservation{ReservationId: "r-1"}})
	if err != nil || released.GetStock().GetAvailable() != 5 {
		t.Fatalf("release: stock=%v err=%v", released, err)
	}
}

func TestInventoryRejectsOversell(t *testing.T) {
	i := NewInventory()
	_, _ = i.AdjustStock(context.Background(), &inventoryv1.AdjustStockRequest{ProductId: "p-1", Adjustment: &inventoryv1.StockAdjustment{QuantityDelta: 1}})
	_, err := i.ReserveStock(context.Background(), &inventoryv1.ReserveStockRequest{ProductId: "p-1", Reservation: &inventoryv1.StockReservation{Quantity: 2}})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Store struct{ db *sql.DB }

type OutboxEvent struct {
	ID, AggregateType, AggregateID, EventType string
	Payload                                   []byte
	OccurredAt                                time.Time
}

func Open(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Adjust(ctx context.Context, productID string, delta int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO inventory_stock (product_id, on_hand, reserved, updated_at) VALUES ($1,$2,0,NOW()) ON CONFLICT (product_id) DO UPDATE SET on_hand=inventory_stock.on_hand+$2, updated_at=NOW() WHERE inventory_stock.on_hand+$2 >= inventory_stock.reserved`, productID, delta); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"product_id": productID, "quantity_delta": delta})
	if err != nil {
		return err
	}
	if err = insertEvent(ctx, tx, productID, "InventoryAdjusted", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Reserve(ctx context.Context, productID, reservationID string, quantity int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var available int64
	if err := tx.QueryRowContext(ctx, `SELECT on_hand-reserved FROM inventory_stock WHERE product_id=$1 FOR UPDATE`, productID).Scan(&available); err != nil {
		return err
	}
	if available < quantity {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_reservations (reservation_id, product_id, quantity, created_at) VALUES ($1,$2,$3,NOW())`, reservationID, productID, quantity); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE inventory_stock SET reserved=reserved+$2, updated_at=NOW() WHERE product_id=$1`, productID, quantity); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"product_id": productID, "reservation_id": reservationID, "quantity": quantity})
	if err != nil {
		return err
	}
	if err = insertEvent(ctx, tx, productID, "InventoryReserved", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Release(ctx context.Context, productID, reservationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var quantity int64
	if err := tx.QueryRowContext(ctx, `SELECT quantity FROM inventory_reservations WHERE reservation_id=$1 AND product_id=$2 FOR UPDATE`, reservationID, productID).Scan(&quantity); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM inventory_reservations WHERE reservation_id=$1`, reservationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE inventory_stock SET reserved=reserved-$2, updated_at=$3 WHERE product_id=$1`, productID, quantity, time.Now().UTC()); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"product_id": productID, "reservation_id": reservationID, "quantity": quantity})
	if err != nil {
		return err
	}
	if err = insertEvent(ctx, tx, productID, "InventoryReleased", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func insertEvent(ctx context.Context, tx *sql.Tx, aggregateID, eventType string, payload []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO event_outbox (event_id, aggregate_type, aggregate_id, event_type, payload, occurred_at) VALUES ($1,$2,$3,$4,$5,NOW())`, uuid.New(), "inventory", aggregateID, eventType, payload)
	return err
}

func (s *Store) ClaimPendingOutbox(ctx context.Context, limit int, workerID string, lease time.Duration) ([]OutboxEvent, error) {
	if limit <= 0 || workerID == "" || lease <= 0 {
		return nil, fmt.Errorf("invalid outbox claim parameters")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `WITH candidates AS (SELECT event_id FROM event_outbox WHERE published_at IS NULL AND (claimed_until IS NULL OR claimed_until < NOW()) ORDER BY occurred_at, event_id LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE event_outbox AS event SET claimed_by=$2, claimed_until=NOW() + ($3 * INTERVAL '1 second') FROM candidates WHERE event.event_id=candidates.event_id RETURNING event.event_id, event.aggregate_type, event.aggregate_id, event.event_type, event.payload, event.occurred_at`, limit, workerID, int64(lease/time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) MarkOutboxPublished(ctx context.Context, id, workerID string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET published_at=$3, claimed_by=NULL, claimed_until=NULL WHERE event_id=$1 AND claimed_by=$2 AND published_at IS NULL`, id, workerID, at)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("outbox event %s is no longer leased by %s", id, workerID)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, productID string) (*inventoryv1.Stock, error) {
	stock := &inventoryv1.Stock{ProductId: productID}
	var updated time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT on_hand, reserved, updated_at FROM inventory_stock WHERE product_id=$1`, productID).Scan(&stock.OnHand, &stock.Reserved, &updated); err != nil {
		return nil, err
	}
	stock.Available, stock.UpdatedAt = stock.OnHand-stock.Reserved, timestamppb.New(updated)
	return stock, nil
}

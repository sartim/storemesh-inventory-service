package repository

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Store struct{ db *sql.DB }

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
	_, err := s.db.ExecContext(ctx, `INSERT INTO inventory_stock (product_id, on_hand, reserved, updated_at) VALUES ($1,$2,0,NOW()) ON CONFLICT (product_id) DO UPDATE SET on_hand=inventory_stock.on_hand+$2, updated_at=NOW() WHERE inventory_stock.on_hand+$2 >= inventory_stock.reserved`, productID, delta)
	return err
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
	return tx.Commit()
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

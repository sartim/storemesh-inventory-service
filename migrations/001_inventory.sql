CREATE TABLE IF NOT EXISTS inventory_stock (
    product_id UUID PRIMARY KEY,
    on_hand BIGINT NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    reserved BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0 AND reserved <= on_hand),
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS inventory_reservations (
    reservation_id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES inventory_stock(product_id),
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS inventory_reservations_product_idx
    ON inventory_reservations (product_id);

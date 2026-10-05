# StoreMesh Inventory Service

The Inventory Service owns stock quantities and reservation state. Product
identity and pricing remain owned by Product Service; orders coordinate with
Inventory but do not mutate its data directly.

The initial protobuf contract defines stock reads, stock adjustments, and
reservation/release operations. Runtime implementation, persistence, and
deployment follow after contract review. The initial PostgreSQL schema is
available in `migrations/001_inventory.sql`; reservation rows are linked to
stock rows and enforce positive quantities.

## Run locally without Docker or Kubernetes

Requires Go 1.26.6 or newer. With no `DATABASE_URL`, the service uses its
in-memory store, which is useful for contract and service development. Run the
process from this repository:

```sh
go run ./cmd/server
```

To run it beside another local StoreMesh service, change its addresses without
changing code:

```sh
GRPC_ADDR=:50052 METRICS_ADDR=:8082 go run ./cmd/server
```

Set `DATABASE_URL` only when testing PostgreSQL-backed behavior; apply
`migrations/001_inventory.sql` and `migrations/002_outbox.sql` first. The
process exposes gRPC on `GRPC_ADDR` and Prometheus metrics on `METRICS_ADDR`.

Persistent stock adjustments and reservation changes also write transactional
`InventoryAdjusted`, `InventoryReserved`, and `InventoryReleased` records to
`event_outbox`. The optional `cmd/outbox-publisher` claims rows with a
30-second PostgreSQL lease and publishes them to `storemesh.inventory.events`.
Delivery is at-least-once, so consumers must be idempotent. Kafka is not
required for local service development and the default Helm deployment does
not run the publisher; enable it only with an external `KAFKA_BROKERS` value.

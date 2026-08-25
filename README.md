# StoreMesh Inventory Service

The Inventory Service owns stock quantities and reservation state. Product
identity and pricing remain owned by Product Service; orders coordinate with
Inventory but do not mutate its data directly.

The initial protobuf contract defines stock reads, stock adjustments, and
reservation/release operations. Runtime implementation, persistence, and
deployment follow after contract review.

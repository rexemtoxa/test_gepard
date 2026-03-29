# gepard_billing

`gepard_billing` is a Go gRPC billing service backed by PostgreSQL. It accepts idempotent billing operations, stores each request result in `operation_requests`, appends applied balance changes to an append-only `ledger_entries` chain, and runs a scheduled worker that cancels the latest odd applied ledger entries when the reversal keeps the balance non-negative.

## Open questions and current assumptions

In a real-world task, I would clarify a few product and domain points before locking the behavior down. To avoid blocking on extra back-and-forth, the current implementation makes the following assumptions explicitly:

- `CancelLatestOddOperations(10)` is interpreted as "inspect the latest 10 eligible odd `apply` entries" rather than "guarantee 10 cancellations per run". If one candidate cannot be canceled because its reversal would make the balance negative, the worker skips that candidate and continues with the next one from the fetched set.
- "Latest odd operations" is interpreted as `apply` ledger rows whose `ledger_entries.id` is odd and that do not already have a matching `cancel` row. It is not interpreted as odd amounts or as every second business operation by request order.
- Idempotency is keyed only by `tx_id`. If the same `tx_id` is sent again with different `source`, `state`, or `amount`, the service returns the original stored result with `duplicate=true` instead of treating it as a conflict.
- Rejected withdrawals are persisted in `operation_requests` for idempotency. If the same `tx_id` is retried later after the balance has increased, the service still returns the original rejection instead of re-evaluating the operation.
- The service currently models a single shared balance. Because the contract has no customer, wallet, or account identifier, deposits, withdrawals, and cancellations all apply to one global ledger.

If the intended business flow differs on any of those points, the cancellation and idempotency rules should be adjusted first, because they directly affect persisted data and replay behavior.

## What the service exposes

- `billing.v1.BillingService/ApplyOperation`
- gRPC health checks
- gRPC reflection

## Requirements

The project requires Go `1.26.1` exactly. The canonical local pin is stored in `.go-version`.

Required tools:

- Task
- dbmate
- Docker

Docker is part of the normal development flow:

- `task gen:proto` runs Buf in Docker
- `task gen:sqlc` runs sqlc in Docker
- `task run`, `task test`, and `task verify` regenerate code before running

## Quick start

1. Copy the example environment file:

```bash
cp .env.example .env
```

2. Verify local tooling and Go version:

```bash
task install
```

3. Start PostgreSQL, apply migrations, generate code, and run the gRPC server:

```bash
task start
```

4. Run the Go test suite:

```bash
task test
```

The server listens on `localhost:50051` by default.

## Environment

The example file is [`.env.example`](.env.example).

| Variable                | Default                                                                      | Purpose                                                                                           |
| ----------------------- | ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `POSTGRES_DB`           | `gepard_billing`                                                             | Local PostgreSQL database name for Docker Compose                                                 |
| `POSTGRES_USER`         | `postgres`                                                                   | Local PostgreSQL user                                                                             |
| `POSTGRES_PASSWORD`     | `postgres`                                                                   | Local PostgreSQL password                                                                         |
| `DATABASE_URL`          | `postgres://postgres:postgres@localhost:5432/gepard_billing?sslmode=disable` | DSN used by local `task` commands and `go run`                                                    |
| `APP_DATABASE_URL`      | `postgres://postgres:postgres@postgres:5432/gepard_billing?sslmode=disable`  | DSN used only by the Compose `app` service                                                        |
| `GRPC_PORT`             | `50051`                                                                      | gRPC listen port                                                                                  |
| `GRPC_UNARY_TIMEOUT`    | `5s`                                                                         | Unary business RPC timeout; set `0` to disable                                                    |
| `SERVICE_NAME`          | `gepard-billing`                                                             | Health service name                                                                               |
| `LOG_LEVEL`             | `INFO`                                                                       | Reserved runtime log level setting                                                                |
| `CANCEL_ODD_OPS_CRON`   | `*/1 * * * *` in `.env.example`                                              | Cancellation worker schedule; standard 5-field cron and descriptors like `@every 1m` are accepted |
| `DB_MAX_OPEN_CONNS`     | `10`                                                                         | Maximum open DB connections                                                                       |
| `DB_MAX_IDLE_CONNS`     | `10`                                                                         | Maximum idle DB connections                                                                       |
| `DB_CONN_MAX_IDLE_TIME` | `5m`                                                                         | Maximum idle connection lifetime                                                                  |
| `DB_CONN_MAX_LIFETIME`  | `30m`                                                                        | Maximum total connection lifetime                                                                 |

## gRPC contract

Proto source: [`proto/billing/v1/billing.proto`](proto/billing/v1/billing.proto)

### `billing.v1.BillingService/ApplyOperation`

Request fields:

- `source`: one of `game`, `payment`, `service`
- `state`: one of `deposit`, `withdraw`
- `amount`: positive plain decimal string such as `10`, `10.15`, or `0.001`
- `tx_id`: idempotency key

Validation notes:

- `amount` must be a plain decimal string
- exponential notation such as `1e3` is rejected
- signed `+10` input is rejected
- zero and negative amounts are rejected

Response fields:

- `result_status`: `APPLIED` or `REJECTED_INSUFFICIENT_FUNDS`
- `duplicate`: `true` when the `tx_id` already exists

Idempotency behavior:

- duplicate detection is keyed only by `tx_id`
- if `tx_id` already exists, the service returns the original stored status with `duplicate=true`
- duplicate requests do not append new ledger rows

Example request:

```bash
grpcurl -plaintext \
  -d '{"source":"game","state":"deposit","amount":"10.15","tx_id":"tx-1"}' \
  localhost:50051 \
  billing.v1.BillingService/ApplyOperation
```

Manual local request samples live in [`internal/billing/doc/_grpc.http`](internal/billing/doc/_grpc.http).

## Data model

The current schema is defined by the migrations in [`db/migrations`](db/migrations) and refreshed into [`db/schema.sql`](db/schema.sql).

### `operation_requests`

One row per `tx_id` stores the normalized request outcome:

- `tx_id` primary key
- `source`: `game`, `payment`, `service`
- `state`: `deposit`, `withdraw`
- `amount`: positive `numeric`
- `result_status`: `applied`, `rejected_insufficient_funds`
- `created_at`

Rejected withdrawals are still recorded here, but they do not create ledger entries.

### `ledger_entries`

This is an append-only linked ledger:

- `id` bigserial primary key
- `request_tx_id` links an `apply` entry to `operation_requests(tx_id)`
- `reverses_entry_id` links a `cancel` entry to the entry it reverses
- `entry_type`: `apply` or `cancel`
- `signed_amount`: positive for deposits, negative for withdrawals, opposite sign for cancellations
- `prev_entry_id`: points to the previous ledger head
- `balance_after`: resulting balance after the entry
- `created_at`

Important constraints:

- `balance_after >= 0`
- an `apply` row must have `request_tx_id` and must not have `reverses_entry_id`
- a `cancel` row must have `reverses_entry_id` and must not have `request_tx_id`
- `request_tx_id`, `reverses_entry_id`, and `prev_entry_id` are unique where present

### Worker-specific index

[`db/migrations/20260329203749_add_index_for_cron.sql`](db/migrations/20260329203749_add_index_for_cron.sql) adds:

- `ledger_entries_apply_odd_latest_idx`

It accelerates the worker query that scans the latest odd `apply` entries in descending `id` order and reads `signed_amount` without a full table scan.

## Cancellation worker

The worker runs on `CANCEL_ODD_OPS_CRON` and calls `CancelLatestOddOperations` with a limit of `10` entries per run.

Current behavior:

- acquires an advisory lock so only one worker run proceeds at a time
- locks balance mutations before scanning candidates
- looks at the latest odd `apply` ledger entries that have not already been canceled
- appends `cancel` entries only when the reversal would keep the balance non-negative
- runs each worker execution with a `15s` timeout

## Run in Docker

Build the image:

```bash
docker build -t gepard-billing:local .
```

Run it with the local PostgreSQL container:

```bash
cp .env.example .env
docker compose up -d postgres
task migrate:up
docker compose up --build app
```

Notes:

- `DATABASE_URL` is the host-oriented DSN used by local `task` commands
- `APP_DATABASE_URL` is the container-oriented DSN used by the Compose `app` service
- the runtime image does not run migrations automatically on startup

## Canonical tasks

### Development

- `task` or `task default`: list available tasks
- `task install`: verify local prerequisites and bootstrap Go dependencies
- `task run`: generate code and start the gRPC server
- `task start`: start PostgreSQL, apply migrations, and run the server
- `task clean`: stop local infra and remove repo-local caches

### Protobuf and sqlc

- `task lint:proto`: run Buf lint
- `task gen:proto`: generate Go code from protobuf definitions
- `task gen:sqlc`: generate Go repository code from SQL

### Database

- `task db:start`: start local PostgreSQL in Docker
- `task db:stop`: stop local PostgreSQL containers
- `task db:reset`: recreate local PostgreSQL and re-apply migrations
- `task db:dump-schema`: refresh `db/schema.sql` from the local container
- `task migrate:new NAME=create_table`: create a new dbmate migration
- `task migrate:up`: apply pending migrations and refresh `db/schema.sql`
- `task migrate:down`: roll back the latest migration and refresh `db/schema.sql`
- `task migrate:status`: show dbmate migration status

### Verification

- `task test`: generate code and run `go test ./...`
- `task verify`: run proto lint, regenerate code, and run `go test ./...`

## Generated code

Generated artifacts are part of the local workflow:

- protobuf Go bindings are generated under [`proto/billing/v1`](proto/billing/v1)
- sqlc repository code is generated under [`internal/repository`](internal/repository)

Do not edit `*.pb.go` or sqlc output manually. Update the source files and rerun the appropriate task instead.

## Project layout

- [`cmd/server`](cmd/server): service entrypoint
- [`internal/billing`](internal/billing): request validation, service logic, gRPC handler, and manual request docs
- [`internal/billingworker`](internal/billingworker): cron scheduler for cancellation jobs
- [`internal/config`](internal/config): environment-driven runtime config
- [`internal/grpcServer`](internal/grpcServer): gRPC server wiring, interceptors, health, and reflection
- [`internal/repository`](internal/repository): generated sqlc repository layer
- [`proto/billing/v1`](proto/billing/v1): protobuf source and generated gRPC bindings
- [`db/migrations`](db/migrations): dbmate migrations
- [`db/query`](db/query): sqlc SQL queries
- [`db/schema.sql`](db/schema.sql): schema dump refreshed from local PostgreSQL

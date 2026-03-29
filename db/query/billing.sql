-- name: LockBalanceMutations :exec
SELECT pg_advisory_xact_lock(sqlc.arg(lock_group)::integer, sqlc.arg(lock_key)::integer);

-- name: TryLockCancellationJob :one
SELECT pg_try_advisory_xact_lock(sqlc.arg(lock_group)::integer, sqlc.arg(lock_key)::integer) AS acquired;

-- name: GetOperationRequestResultStatus :one
SELECT
  result_status
FROM operation_requests
WHERE tx_id = sqlc.arg(tx_id);

-- name: InsertOperationRequest :one
INSERT INTO operation_requests (
  tx_id,
  source,
  state,
  amount,
  result_status
) VALUES (
  sqlc.arg(tx_id),
  sqlc.arg(source),
  sqlc.arg(state),
  sqlc.arg(amount)::numeric,
  sqlc.arg(result_status)
)
RETURNING
  tx_id,
  source,
  state,
  amount::text AS amount_text,
  result_status,
  created_at;

-- name: GetLedgerHead :one
SELECT
  COALESCE((SELECT id FROM ledger_entries ORDER BY id DESC LIMIT 1), 0)::bigint AS id,
  COALESCE((SELECT balance_after::text FROM ledger_entries ORDER BY id DESC LIMIT 1), '0')::text AS balance_after_text;

-- name: InsertApplyLedgerEntry :one
INSERT INTO ledger_entries (
  request_tx_id,
  entry_type,
  signed_amount,
  prev_entry_id,
  balance_after
) VALUES (
  sqlc.arg(request_tx_id),
  'apply',
  sqlc.arg(signed_amount)::numeric,
  sqlc.narg(prev_entry_id)::bigint,
  sqlc.arg(balance_after)::numeric
)
RETURNING
  id,
  request_tx_id,
  reverses_entry_id,
  entry_type,
  signed_amount::text AS signed_amount_text,
  prev_entry_id,
  balance_after::text AS balance_after_text,
  created_at;

-- name: InsertCancelLedgerEntry :one
INSERT INTO ledger_entries (
  reverses_entry_id,
  entry_type,
  signed_amount,
  prev_entry_id,
  balance_after
) VALUES (
  sqlc.arg(reverses_entry_id),
  'cancel',
  sqlc.arg(signed_amount)::numeric,
  sqlc.narg(prev_entry_id)::bigint,
  sqlc.arg(balance_after)::numeric
)
RETURNING
  id,
  request_tx_id,
  reverses_entry_id,
  entry_type,
  signed_amount::text AS signed_amount_text,
  prev_entry_id,
  balance_after::text AS balance_after_text,
  created_at;

-- name: ListCancelCandidates :many
SELECT le.id, le.signed_amount::text
FROM ledger_entries AS le
LEFT JOIN ledger_entries AS canceled
  ON canceled.reverses_entry_id = le.id
 AND canceled.entry_type = 'cancel'
WHERE le.entry_type = 'apply'
  AND le.id % 2 = 1
  AND canceled.id IS NULL
ORDER BY le.id DESC
LIMIT sqlc.arg(limit_count);

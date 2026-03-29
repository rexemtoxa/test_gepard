-- migrate:up
CREATE INDEX IF NOT EXISTS ledger_entries_apply_odd_latest_idx
ON ledger_entries (id DESC)
INCLUDE (signed_amount)
WHERE entry_type = 'apply'
  AND (id % 2 = 1);

-- migrate:down
DROP INDEX IF EXISTS ledger_entries_apply_odd_latest_idx;

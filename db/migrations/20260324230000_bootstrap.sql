-- migrate:up

CREATE TABLE IF NOT EXISTS operation_requests (
    tx_id text PRIMARY KEY,
    source text NOT NULL CHECK (source IN ('game', 'payment', 'service')),
    state text NOT NULL CHECK (state IN ('deposit', 'withdraw')),
    amount numeric NOT NULL CHECK (amount > 0),
    result_status text NOT NULL CHECK (result_status IN ('applied', 'rejected_insufficient_funds')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id bigserial PRIMARY KEY,
    request_tx_id text NULL UNIQUE REFERENCES operation_requests(tx_id),
    reverses_entry_id bigint NULL UNIQUE REFERENCES ledger_entries(id),
    entry_type text NOT NULL CHECK (entry_type IN ('apply', 'cancel')),
    signed_amount numeric NOT NULL,
    prev_entry_id bigint NULL UNIQUE REFERENCES ledger_entries(id),
    balance_after numeric NOT NULL CHECK (balance_after >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (entry_type = 'apply' AND request_tx_id IS NOT NULL AND reverses_entry_id IS NULL) OR
        (entry_type = 'cancel' AND request_tx_id IS NULL AND reverses_entry_id IS NOT NULL)
    )
);

-- migrate:down

DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS operation_requests;

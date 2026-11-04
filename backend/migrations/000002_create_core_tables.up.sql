CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'analyst' CHECK (role IN ('admin', 'analyst', 'reviewer')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bank_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    bank_name TEXT NOT NULL,
    account_number_masked TEXT NOT NULL DEFAULT '',
    currency CHAR(3) NOT NULL DEFAULT 'IDR',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bank_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id UUID NOT NULL REFERENCES bank_accounts(id),
    external_id TEXT,
    transaction_date DATE NOT NULL,
    amount NUMERIC(20, 4) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'IDR',
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('debit', 'credit')),
    description TEXT NOT NULL DEFAULT '',
    reference TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'imported' CHECK (status IN ('imported', 'matched', 'exception')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bank_account_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_bank_transactions_date
    ON bank_transactions (transaction_date DESC);
CREATE INDEX IF NOT EXISTS idx_bank_transactions_status
    ON bank_transactions (status);

CREATE TABLE IF NOT EXISTS reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    started_by UUID REFERENCES users(id),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (period_end >= period_start)
);

CREATE TABLE IF NOT EXISTS reconciliation_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reconciliation_run_id UUID NOT NULL REFERENCES reconciliation_runs(id) ON DELETE CASCADE,
    bank_transaction_id UUID NOT NULL REFERENCES bank_transactions(id),
    match_key TEXT NOT NULL,
    confidence NUMERIC(5, 2) NOT NULL DEFAULT 100.00 CHECK (confidence BETWEEN 0 AND 100),
    matched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (reconciliation_run_id, bank_transaction_id)
);

CREATE TABLE IF NOT EXISTS reconciliation_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_transaction_id UUID NOT NULL UNIQUE REFERENCES bank_transactions(id),
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'resolved', 'waived')),
    assigned_to UUID REFERENCES users(id),
    resolution_note TEXT NOT NULL DEFAULT '',
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reconciliation_exceptions_status
    ON reconciliation_exceptions (status, created_at DESC);

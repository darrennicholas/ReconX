CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_key TEXT NOT NULL UNIQUE
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id),
    permission_id UUID NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role_id UUID NOT NULL REFERENCES roles(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash BYTEA NOT NULL UNIQUE,
    csrf_token_hash BYTEA NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_active_idx ON sessions(user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE bank_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_code TEXT NOT NULL UNIQUE,
    bank_display_name TEXT NOT NULL,
    masked_account_no TEXT NOT NULL,
    account_holder_name TEXT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'IDR' CHECK (currency = 'IDR'),
    timezone TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reconciliation_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_no TEXT NOT NULL UNIQUE,
    account_id UUID NOT NULL REFERENCES bank_accounts(id),
    period_from DATE NOT NULL,
    period_to DATE NOT NULL,
    remarks TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'DRAFT' CHECK (state IN ('DRAFT','READY','PROCESSING','REVIEW','FAILED','PENDING_APPROVAL','APPROVED','CLOSED','CANCELLED')),
    source_version BIGINT NOT NULL DEFAULT 1 CHECK (source_version > 0),
    row_version BIGINT NOT NULL DEFAULT 1 CHECK (row_version > 0),
    created_by UUID NOT NULL REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (period_from <= period_to)
);
CREATE INDEX batches_account_period_idx ON reconciliation_batches(account_id, period_from, period_to);
CREATE INDEX batches_state_updated_idx ON reconciliation_batches(state, updated_at DESC);

CREATE TABLE import_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    source_type TEXT NOT NULL CHECK (source_type IN ('INTERNAL','BANK')),
    source_version BIGINT NOT NULL,
    sha256 BYTEA NOT NULL,
    original_name TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    error_report_key TEXT,
    state TEXT NOT NULL CHECK (state IN ('UPLOADED','VALIDATING','VALID','INVALID','SUPERSEDED')),
    rows_count INTEGER NOT NULL DEFAULT 0 CHECK (rows_count >= 0),
    imported_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (batch_id, source_type, source_version, sha256)
);
CREATE UNIQUE INDEX import_files_one_valid_source_idx ON import_files(batch_id, source_type) WHERE state = 'VALID';

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    import_file_id UUID NOT NULL REFERENCES import_files(id),
    source_type TEXT NOT NULL CHECK (source_type IN ('INTERNAL','BANK')),
    reference_raw TEXT NOT NULL,
    reference_canonical TEXT NOT NULL,
    transaction_date DATE NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('CREDIT','DEBIT')),
    amount_rupiah BIGINT NOT NULL CHECK (amount_rupiah > 0),
    currency CHAR(3) NOT NULL CHECK (currency = 'IDR'),
    source_version BIGINT NOT NULL,
    raw_row JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (import_file_id, reference_canonical)
);
CREATE INDEX transactions_batch_version_idx ON transactions(batch_id, source_version, source_type);
CREATE INDEX transactions_reference_date_idx ON transactions(reference_canonical, transaction_date);

CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    source_version BIGINT NOT NULL,
    rule_version TEXT NOT NULL DEFAULT 'M01',
    state TEXT NOT NULL CHECK (state IN ('PROCESSING','SUCCEEDED','FAILED','ARCHIVED')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    error TEXT,
    UNIQUE (id, batch_id, source_version)
);
CREATE UNIQUE INDEX runs_one_processing_per_batch_idx ON reconciliation_runs(batch_id) WHERE state = 'PROCESSING';

CREATE TABLE approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('MANUAL_MATCH','BATCH_CLOSE')),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    exception_id UUID,
    run_id UUID REFERENCES reconciliation_runs(id),
    source_version BIGINT NOT NULL,
    maker_id UUID NOT NULL REFERENCES users(id),
    checker_id UUID REFERENCES users(id),
    state TEXT NOT NULL CHECK (state IN ('PENDING','APPROVED','REJECTED','INVALIDATED')),
    match_proposal_payload JSONB,
    reason TEXT NOT NULL DEFAULT '',
    decision_comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    CHECK (checker_id IS NULL OR checker_id <> maker_id)
);
CREATE INDEX approvals_state_created_idx ON approval_requests(state, created_at DESC);
CREATE INDEX approvals_batch_idx ON approval_requests(batch_id, created_at DESC);

CREATE TABLE transaction_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    run_id UUID NOT NULL REFERENCES reconciliation_runs(id),
    internal_transaction_id UUID NOT NULL REFERENCES transactions(id),
    bank_transaction_id UUID NOT NULL REFERENCES transactions(id),
    method TEXT NOT NULL CHECK (method IN ('AUTO_MATCHED','MANUAL_MATCHED')),
    approval_id UUID REFERENCES approval_requests(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (internal_transaction_id <> bank_transaction_id),
    CHECK ((method = 'AUTO_MATCHED' AND approval_id IS NULL) OR (method = 'MANUAL_MATCHED' AND approval_id IS NOT NULL))
);
CREATE UNIQUE INDEX matches_active_internal_idx ON transaction_matches(internal_transaction_id) WHERE active;
CREATE UNIQUE INDEX matches_active_bank_idx ON transaction_matches(bank_transaction_id) WHERE active;

CREATE TABLE reconciliation_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES reconciliation_batches(id),
    run_id UUID NOT NULL REFERENCES reconciliation_runs(id),
    type TEXT NOT NULL CHECK (type IN ('AMOUNT_MISMATCH','DIRECTION_MISMATCH','DATE_VARIANCE','AMBIGUOUS_MATCH','INTERNAL_ONLY','BANK_ONLY')),
    state TEXT NOT NULL DEFAULT 'OPEN' CHECK (state IN ('OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL','RESOLVED','ARCHIVED')),
    internal_transaction_id UUID REFERENCES transactions(id),
    bank_transaction_id UUID REFERENCES transactions(id),
    assigned_to UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    CHECK (internal_transaction_id IS NOT NULL OR bank_transaction_id IS NOT NULL)
);
CREATE INDEX exceptions_batch_state_idx ON reconciliation_exceptions(batch_id, state, created_at DESC);
CREATE INDEX exceptions_assignee_idx ON reconciliation_exceptions(assigned_to, state);

ALTER TABLE approval_requests ADD CONSTRAINT approvals_exception_fk FOREIGN KEY (exception_id) REFERENCES reconciliation_exceptions(id);

CREATE TABLE exception_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exception_id UUID NOT NULL REFERENCES reconciliation_exceptions(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    action_type TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    evidence_key TEXT,
    evidence_filename TEXT,
    evidence_content_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX exception_actions_timeline_idx ON exception_actions(exception_id, created_at);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id),
    action TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    before_json JSONB,
    after_json JSONB,
    request_id TEXT NOT NULL,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs(created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs(entity, entity_id, created_at DESC);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    target_entity TEXT NOT NULL DEFAULT '',
    target_entity_id TEXT NOT NULL DEFAULT '',
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_created_idx ON notifications(user_id, created_at DESC);

CREATE TABLE settings (
    setting_key TEXT PRIMARY KEY,
    setting_value JSONB NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency_keys (
    user_id UUID NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL,
    request_path TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    response_status INTEGER NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, idempotency_key)
);

CREATE OR REPLACE FUNCTION reconx_reject_immutable_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'append-only record cannot be changed' USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs FOR EACH ROW EXECUTE FUNCTION reconx_reject_immutable_change();
CREATE TRIGGER exception_actions_append_only BEFORE UPDATE OR DELETE ON exception_actions FOR EACH ROW EXECUTE FUNCTION reconx_reject_immutable_change();

INSERT INTO roles(code, name) VALUES
    ('ADMIN', 'Admin'), ('OPERATOR', 'Operator'), ('CHECKER', 'Checker'), ('AUDITOR', 'Auditor');

INSERT INTO permissions(permission_key) VALUES
    ('users.read'), ('users.manage'), ('accounts.read'), ('accounts.manage'),
    ('batches.read'), ('batches.create'), ('batches.edit'), ('imports.manage'),
    ('matching.run'), ('exceptions.investigate'), ('matches.propose'), ('matches.approve'),
    ('batches.submit'), ('batches.approve'), ('batches.close'), ('reports.read'),
    ('reports.export'), ('audit.read'), ('settings.manage');

INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE (r.code = 'ADMIN' AND p.permission_key IN ('users.read','users.manage','accounts.read','accounts.manage','batches.read','reports.read','reports.export','audit.read','settings.manage'))
   OR (r.code = 'OPERATOR' AND p.permission_key IN ('accounts.read','batches.read','batches.create','batches.edit','imports.manage','matching.run','exceptions.investigate','matches.propose','batches.submit','reports.read','reports.export'))
   OR (r.code = 'CHECKER' AND p.permission_key IN ('accounts.read','batches.read','matches.approve','batches.approve','batches.close','reports.read','reports.export','audit.read'))
   OR (r.code = 'AUDITOR' AND p.permission_key IN ('accounts.read','batches.read','reports.read','reports.export','audit.read'));

INSERT INTO settings(setting_key, setting_value) VALUES
    ('timezone', '"Asia/Jakarta"'),
    ('max_import_bytes', '10485760'),
    ('csv_delimiter', '","'),
    ('matching_rule', '{"id":"M01","mode":"exact","version":1}'),
    ('manual_date_tolerance_days', '3');

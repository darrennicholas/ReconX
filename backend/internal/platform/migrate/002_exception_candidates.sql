ALTER TABLE reconciliation_exceptions
    ADD COLUMN candidate_transaction_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

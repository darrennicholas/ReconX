-- Multiple transactions may share a reference; matching flags them for review.
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_import_file_id_reference_canonical_key;

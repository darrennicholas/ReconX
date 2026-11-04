package reconciliation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reconx/backend/internal/platform/audit"
)

var ErrDuplicateImport = errors.New("duplicate file for this batch and source")

type ImportView struct {
	ID            string           `json:"id"`
	BatchID       string           `json:"batch_id"`
	SourceType    string           `json:"source_type"`
	SourceVersion int64            `json:"source_version"`
	OriginalName  string           `json:"original_name"`
	State         string           `json:"state"`
	RowsCount     int              `json:"rows_count"`
	ErrorReport   bool             `json:"has_error_report"`
	CreatedAt     time.Time        `json:"created_at"`
	Errors        []ImportRowError `json:"errors,omitempty"`
}

type StoredArtifact struct {
	Path     string
	Filename string
	MimeType string
}

func (s Service) UploadCSV(ctx context.Context, batchID, source, filename string, data []byte, actorID, requestID, ip string) (ImportView, error) {
	source = strings.ToUpper(strings.TrimSpace(source))
	filename = filepath.Base(strings.TrimSpace(filename))
	if source != "INTERNAL" && source != "BANK" {
		return ImportView{}, ErrCSVInvalid
	}
	if filename == "" || filename == "." || len(data) == 0 || len(data) > MaxImportBytes {
		return ImportView{}, ErrCSVInvalid
	}
	batch, err := s.GetBatch(ctx, batchID)
	if err != nil {
		return ImportView{}, err
	}
	if batch.State != "DRAFT" {
		return ImportView{}, ErrBatchConflict
	}
	hash := sha256.Sum256(data)
	var duplicate bool
	if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM import_files WHERE batch_id=$1 AND source_type=$2 AND sha256=$3)`, batchID, source, hash[:]).Scan(&duplicate); err != nil {
		return ImportView{}, err
	}
	if duplicate {
		return ImportView{}, ErrDuplicateImport
	}

	limit, err := s.importLimit(ctx)
	if err != nil {
		return ImportView{}, err
	}
	parsed, parseErr := ParsedImport{}, error(nil)
	if int64(len(data)) > limit {
		parsed.Errors = []ImportRowError{{Line: 1, Field: "file", Message: fmt.Sprintf("File exceeds the configured %d-byte limit", limit)}}
		parseErr = ErrCSVInvalid
	} else {
		parsed, parseErr = ParseImportCSV(data, batch.PeriodFrom, batch.PeriodTo)
	}
	if parseErr != nil && len(parsed.Errors) == 0 {
		parsed.Errors = []ImportRowError{{Line: 1, Field: "file", Message: parseErr.Error()}}
	}
	state := "VALID"
	if parseErr != nil || len(parsed.Errors) > 0 || len(parsed.Transactions) == 0 {
		state = "INVALID"
	}
	storageKey, err := storePrivate(s.StoragePath, data, ".csv")
	if err != nil {
		return ImportView{}, err
	}
	cleanup := []string{privatePath(s.StoragePath, storageKey)}
	var errorKey any
	if state == "INVALID" {
		errorKey, err = storePrivate(s.StoragePath, ErrorCSV(parsed.Errors), ".errors.csv")
		if err != nil {
			_ = os.Remove(cleanup[0])
			return ImportView{}, err
		}
		cleanup = append(cleanup, privatePath(s.StoragePath, errorKey.(string)))
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	defer tx.Rollback()
	var currentState string
	var sourceVersion int64
	var from, to time.Time
	if err := tx.QueryRowContext(ctx, `SELECT state,source_version,period_from,period_to FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&currentState, &sourceVersion, &from, &to); err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	if currentState != "DRAFT" || sourceVersion != batch.SourceVersion || from != batch.PeriodFrom || to != batch.PeriodTo {
		removePrivate(cleanup)
		return ImportView{}, ErrBatchConflict
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM import_files WHERE batch_id=$1 AND source_type=$2 AND sha256=$3)`, batchID, source, hash[:]).Scan(&duplicate); err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	if duplicate {
		removePrivate(cleanup)
		return ImportView{}, ErrDuplicateImport
	}
	if state == "VALID" {
		if _, err := tx.ExecContext(ctx, `UPDATE import_files SET state='SUPERSEDED' WHERE batch_id=$1 AND source_type=$2 AND state='VALID'`, batchID, source); err != nil {
			removePrivate(cleanup)
			return ImportView{}, err
		}
	}
	var created ImportView
	err = tx.QueryRowContext(ctx, `INSERT INTO import_files(batch_id,source_type,source_version,sha256,original_name,storage_key,error_report_key,state,rows_count,imported_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id::text,batch_id::text,source_type,source_version,original_name,state,rows_count,(error_report_key IS NOT NULL),created_at`, batchID, source, sourceVersion, hash[:], filename, storageKey, errorKey, state, len(parsed.Transactions), actorID).
		Scan(&created.ID, &created.BatchID, &created.SourceType, &created.SourceVersion, &created.OriginalName, &created.State, &created.RowsCount, &created.ErrorReport, &created.CreatedAt)
	if err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	if state == "VALID" {
		if err := insertTransactions(ctx, tx, batchID, created.ID, source, sourceVersion, parsed.Transactions); err != nil {
			removePrivate(cleanup)
			return ImportView{}, err
		}
		var validSources int
		if err := tx.QueryRowContext(ctx, `SELECT count(DISTINCT source_type) FROM import_files WHERE batch_id=$1 AND source_version=$2 AND state='VALID'`, batchID, sourceVersion).Scan(&validSources); err != nil {
			removePrivate(cleanup)
			return ImportView{}, err
		}
		if validSources == 2 {
			if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='READY',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
				removePrivate(cleanup)
				return ImportView{}, err
			}
		}
	} else {
		created.Errors = parsed.Errors
		if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) VALUES ($1,'IMPORT_FAILED','Import CSV gagal','Periksa laporan validasi dan unggah file yang diperbaiki.','batch',$2)`, actorID, batchID); err != nil {
			removePrivate(cleanup)
			return ImportView{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "IMPORT_VALIDATED", "import", created.ID, nil, map[string]any{"batch_id": batchID, "source_type": source, "state": state, "rows_count": created.RowsCount}, requestID, ip); err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	if err := tx.Commit(); err != nil {
		removePrivate(cleanup)
		return ImportView{}, err
	}
	return created, nil
}

func (s Service) importLimit(ctx context.Context) (int64, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT setting_value FROM settings WHERE setting_key='max_import_bytes'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return MaxImportBytes, nil
	}
	if err != nil {
		return 0, err
	}
	var limit int64
	if err := json.Unmarshal(raw, &limit); err != nil || limit < 1024 || limit > MaxImportBytes {
		return MaxImportBytes, nil
	}
	return limit, nil
}

func (s Service) ListImports(ctx context.Context, batchID string) ([]ImportView, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text,batch_id::text,source_type,source_version,original_name,state,rows_count,(error_report_key IS NOT NULL),created_at FROM import_files WHERE batch_id=$1 ORDER BY source_version,created_at DESC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ImportView, 0)
	for rows.Next() {
		var item ImportView
		if err := rows.Scan(&item.ID, &item.BatchID, &item.SourceType, &item.SourceVersion, &item.OriginalName, &item.State, &item.RowsCount, &item.ErrorReport, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Service) ErrorReport(ctx context.Context, importID string) (StoredArtifact, error) {
	var key, name sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT error_report_key,original_name FROM import_files WHERE id=$1`, importID).Scan(&key, &name)
	if err != nil {
		return StoredArtifact{}, err
	}
	if !key.Valid || key.String == "" {
		return StoredArtifact{}, sql.ErrNoRows
	}
	path := privatePath(s.StoragePath, key.String)
	if _, err := os.Stat(path); err != nil {
		return StoredArtifact{}, err
	}
	return StoredArtifact{Path: path, Filename: name.String + ".errors.csv", MimeType: "text/csv"}, nil
}

func (s Service) ImportArtifact(ctx context.Context, importID string) (StoredArtifact, error) {
	var key, name string
	err := s.DB.QueryRowContext(ctx, `SELECT storage_key,original_name FROM import_files WHERE id=$1`, importID).Scan(&key, &name)
	if err != nil {
		return StoredArtifact{}, err
	}
	path := privatePath(s.StoragePath, key)
	if _, err := os.Stat(path); err != nil {
		return StoredArtifact{}, err
	}
	return StoredArtifact{Path: path, Filename: filepath.Base(name), MimeType: "text/csv"}, nil
}

func insertTransactions(ctx context.Context, tx *sql.Tx, batchID, importID, source string, sourceVersion int64, rows []ImportedTransaction) error {
	const chunkSize = 500
	for start := 0; start < len(rows); start += chunkSize {
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		values := make([]any, 0, (end-start)*10)
		placeholders := make([]string, 0, end-start)
		for index, row := range rows[start:end] {
			base := index * 10
			placeholders = append(placeholders, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d::jsonb)", base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10))
			values = append(values, batchID, importID, source, row.ReferenceRaw, row.ReferenceCanonical, row.Date, row.Direction, row.Amount, sourceVersion, row.RawRow)
		}
		query := `INSERT INTO transactions(batch_id,import_file_id,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,source_version,raw_row) VALUES ` + strings.Join(placeholders, ",")
		if _, err := tx.ExecContext(ctx, query, values...); err != nil {
			return err
		}
	}
	return nil
}

func storePrivate(root string, data []byte, suffix string) (string, error) {
	if root == "" {
		root = filepath.Join("private-data", "imports")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	key := hex.EncodeToString(keyBytes) + suffix
	filePath := filepath.Join(root, key)
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		return "", err
	}
	return key, nil
}

func privatePath(root string, key any) string {
	if root == "" {
		root = filepath.Join("private-data", "imports")
	}
	value, _ := key.(string)
	if value == "" || filepath.Base(value) != value {
		return ""
	}
	return filepath.Join(root, value)
}

func removePrivate(paths []string) {
	for _, path := range paths {
		if path != "" {
			_ = os.Remove(path)
		}
	}
}

func validateImportExists(ctx context.Context, db *sql.DB, importID string) error {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM import_files WHERE id=$1)`, importID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

func validateFileSize(size int64) error {
	if size <= 0 || size > MaxImportBytes {
		return fmt.Errorf("file size exceeds allowed limit")
	}
	return nil
}

func importValidationMessage(err error) string {
	if errors.Is(err, ErrCSVInvalid) {
		return "CSV tidak sesuai format yang diwajibkan."
	}
	return err.Error()
}

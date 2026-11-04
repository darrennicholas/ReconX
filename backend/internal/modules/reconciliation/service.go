package reconciliation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"reconx/backend/internal/platform/audit"
)

var (
	ErrBatchConflict  = errors.New("batch state conflict")
	ErrBatchOverlap   = errors.New("batch period overlaps another batch")
	ErrInvalidPeriod  = errors.New("invalid batch period")
	ErrForbiddenScope = errors.New("resource outside operator scope")
)

type Service struct {
	DB          *sql.DB
	StoragePath string
}

type BatchView struct {
	ID               string    `json:"id"`
	BatchNo          string    `json:"batch_no"`
	AccountID        string    `json:"account_id"`
	AccountCode      string    `json:"account_code"`
	BankDisplayName  string    `json:"bank_display_name"`
	PeriodFrom       time.Time `json:"period_from"`
	PeriodTo         time.Time `json:"period_to"`
	Remarks          string    `json:"remarks"`
	State            string    `json:"state"`
	SourceVersion    int64     `json:"source_version"`
	RowVersion       int64     `json:"row_version"`
	CreatedBy        string    `json:"created_by"`
	MakerName        string    `json:"maker_name"`
	InternalCount    int       `json:"internal_count"`
	BankCount        int       `json:"bank_count"`
	MatchedPairs     int       `json:"matched_pairs"`
	ActiveExceptions int       `json:"active_exceptions"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BatchInput struct {
	AccountID  string
	PeriodFrom time.Time
	PeriodTo   time.Time
	Remarks    string
}

func (s Service) CreateBatch(ctx context.Context, input BatchInput, makerID, requestID, ip string) (BatchView, error) {
	input.Remarks = strings.TrimSpace(input.Remarks)
	if input.AccountID == "" || input.PeriodFrom.IsZero() || input.PeriodTo.IsZero() || input.PeriodFrom.After(input.PeriodTo) {
		return BatchView{}, ErrInvalidPeriod
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchView{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.AccountID); err != nil {
		return BatchView{}, err
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT active FROM bank_accounts WHERE id=$1 FOR UPDATE`, input.AccountID).Scan(&active); err != nil {
		return BatchView{}, err
	}
	if !active {
		return BatchView{}, ErrBatchConflict
	}
	var overlap bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reconciliation_batches WHERE account_id=$1 AND state <> 'CANCELLED' AND period_from <= $3 AND period_to >= $2)`, input.AccountID, input.PeriodFrom, input.PeriodTo).Scan(&overlap); err != nil {
		return BatchView{}, err
	}
	if overlap {
		return BatchView{}, ErrBatchOverlap
	}
	datePrefix := input.PeriodFrom.Format("20060102")
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, datePrefix); err != nil {
		return BatchView{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reconciliation_batches WHERE batch_no LIKE $1`, "REC-"+datePrefix+"-%").Scan(&count); err != nil {
		return BatchView{}, err
	}
	batchNo := fmt.Sprintf("REC-%s-%04d", datePrefix, count+1)
	var created BatchView
	err = tx.QueryRowContext(ctx, `INSERT INTO reconciliation_batches(batch_no,account_id,period_from,period_to,remarks,created_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text,batch_no,account_id::text,period_from,period_to,remarks,state,source_version,row_version,created_by::text,updated_at`, batchNo, input.AccountID, input.PeriodFrom, input.PeriodTo, input.Remarks, makerID).
		Scan(&created.ID, &created.BatchNo, &created.AccountID, &created.PeriodFrom, &created.PeriodTo, &created.Remarks, &created.State, &created.SourceVersion, &created.RowVersion, &created.CreatedBy, &created.UpdatedAt)
	if err != nil {
		return BatchView{}, err
	}
	if err := audit.Append(ctx, tx, makerID, "BATCH_CREATED", "batch", created.ID, nil, map[string]any{"batch_no": created.BatchNo, "state": created.State, "account_id": created.AccountID}, requestID, ip); err != nil {
		return BatchView{}, err
	}
	if err := tx.Commit(); err != nil {
		return BatchView{}, err
	}
	return s.GetBatch(ctx, created.ID)
}

func (s Service) ListBatches(ctx context.Context, userID string, operator bool, q, state, accountID, periodFrom, periodTo string, page, pageSize int) ([]BatchView, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	base := ` FROM reconciliation_batches b JOIN bank_accounts a ON a.id=b.account_id JOIN users u ON u.id=b.created_by
		WHERE ($1='' OR b.batch_no ILIKE '%'||$1||'%' OR a.account_code ILIKE '%'||$1||'%')
		AND ($2='' OR b.state=$2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)
		AND ($6='' OR b.period_from >= $6::date) AND ($7='' OR b.period_to <= $7::date)`
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+base, q, state, accountID, operator, userID, periodFrom, periodTo).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT b.id::text,b.batch_no,b.account_id::text,a.account_code,a.bank_display_name,b.period_from,b.period_to,b.remarks,b.state,b.source_version,b.row_version,b.created_by::text,u.full_name,
		(SELECT count(*) FROM transactions t WHERE t.batch_id=b.id AND t.source_version=b.source_version AND t.source_type='INTERNAL'),
		(SELECT count(*) FROM transactions t WHERE t.batch_id=b.id AND t.source_version=b.source_version AND t.source_type='BANK'),
		(SELECT count(*) FROM transaction_matches m JOIN reconciliation_runs rr ON rr.id=m.run_id WHERE m.batch_id=b.id AND m.active AND rr.source_version=b.source_version),
		(SELECT count(*) FROM reconciliation_exceptions e WHERE e.batch_id=b.id AND e.run_id=(SELECT id FROM reconciliation_runs WHERE batch_id=b.id AND source_version=b.source_version AND state='SUCCEEDED' ORDER BY started_at DESC LIMIT 1) AND e.state NOT IN ('RESOLVED','ARCHIVED')),
		b.updated_at` + base + ` ORDER BY b.created_at DESC LIMIT $8 OFFSET $9`
	rows, err := s.DB.QueryContext(ctx, query, q, state, accountID, operator, userID, periodFrom, periodTo, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]BatchView, 0)
	for rows.Next() {
		var item BatchView
		if err := rows.Scan(&item.ID, &item.BatchNo, &item.AccountID, &item.AccountCode, &item.BankDisplayName, &item.PeriodFrom, &item.PeriodTo, &item.Remarks, &item.State, &item.SourceVersion, &item.RowVersion, &item.CreatedBy, &item.MakerName, &item.InternalCount, &item.BankCount, &item.MatchedPairs, &item.ActiveExceptions, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s Service) GetBatch(ctx context.Context, id string) (BatchView, error) {
	var item BatchView
	err := s.DB.QueryRowContext(ctx, `SELECT b.id::text,b.batch_no,b.account_id::text,a.account_code,a.bank_display_name,b.period_from,b.period_to,b.remarks,b.state,b.source_version,b.row_version,b.created_by::text,u.full_name,
		(SELECT count(*) FROM transactions t WHERE t.batch_id=b.id AND t.source_version=b.source_version AND t.source_type='INTERNAL'),
		(SELECT count(*) FROM transactions t WHERE t.batch_id=b.id AND t.source_version=b.source_version AND t.source_type='BANK'),
		(SELECT count(*) FROM transaction_matches m JOIN reconciliation_runs rr ON rr.id=m.run_id WHERE m.batch_id=b.id AND m.active AND rr.source_version=b.source_version),
		(SELECT count(*) FROM reconciliation_exceptions e WHERE e.batch_id=b.id AND e.run_id=(SELECT id FROM reconciliation_runs WHERE batch_id=b.id AND source_version=b.source_version AND state='SUCCEEDED' ORDER BY started_at DESC LIMIT 1) AND e.state NOT IN ('RESOLVED','ARCHIVED')),
		b.updated_at FROM reconciliation_batches b JOIN bank_accounts a ON a.id=b.account_id JOIN users u ON u.id=b.created_by WHERE b.id=$1`, id).
		Scan(&item.ID, &item.BatchNo, &item.AccountID, &item.AccountCode, &item.BankDisplayName, &item.PeriodFrom, &item.PeriodTo, &item.Remarks, &item.State, &item.SourceVersion, &item.RowVersion, &item.CreatedBy, &item.MakerName, &item.InternalCount, &item.BankCount, &item.MatchedPairs, &item.ActiveExceptions, &item.UpdatedAt)
	return item, err
}

func (s Service) UpdateDraft(ctx context.Context, id string, expectedVersion int64, input BatchInput, actorID, requestID, ip string) (BatchView, error) {
	if input.PeriodFrom.IsZero() || input.PeriodTo.IsZero() || input.PeriodFrom.After(input.PeriodTo) {
		return BatchView{}, ErrInvalidPeriod
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchView{}, err
	}
	defer tx.Rollback()
	var current BatchView
	err = tx.QueryRowContext(ctx, `SELECT id::text,account_id::text,period_from,period_to,remarks,state,row_version,created_by::text FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, id).
		Scan(&current.ID, &current.AccountID, &current.PeriodFrom, &current.PeriodTo, &current.Remarks, &current.State, &current.RowVersion, &current.CreatedBy)
	if err != nil {
		return BatchView{}, err
	}
	if current.State != "DRAFT" || current.RowVersion != expectedVersion || current.CreatedBy != actorID {
		return BatchView{}, ErrBatchConflict
	}
	if input.AccountID == "" {
		input.AccountID = current.AccountID
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.AccountID); err != nil {
		return BatchView{}, err
	}
	var accountActive bool
	if err := tx.QueryRowContext(ctx, `SELECT active FROM bank_accounts WHERE id=$1 FOR UPDATE`, input.AccountID).Scan(&accountActive); err != nil {
		return BatchView{}, err
	}
	if !accountActive {
		return BatchView{}, ErrBatchConflict
	}
	var overlap bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reconciliation_batches WHERE id<>$1 AND account_id=$2 AND state<>'CANCELLED' AND period_from<=$4 AND period_to>=$3)`, id, input.AccountID, input.PeriodFrom, input.PeriodTo).Scan(&overlap); err != nil {
		return BatchView{}, err
	}
	if overlap {
		return BatchView{}, ErrBatchOverlap
	}
	_, err = tx.ExecContext(ctx, `UPDATE reconciliation_batches SET account_id=$2,period_from=$3,period_to=$4,remarks=$5,row_version=row_version+1,updated_at=now() WHERE id=$1`, id, input.AccountID, input.PeriodFrom, input.PeriodTo, strings.TrimSpace(input.Remarks))
	if err != nil {
		return BatchView{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "BATCH_UPDATED", "batch", id, map[string]any{"state": current.State, "period_from": current.PeriodFrom, "period_to": current.PeriodTo}, map[string]any{"state": "DRAFT", "period_from": input.PeriodFrom, "period_to": input.PeriodTo}, requestID, ip); err != nil {
		return BatchView{}, err
	}
	if err := tx.Commit(); err != nil {
		return BatchView{}, err
	}
	return s.GetBatch(ctx, id)
}

func (s Service) CancelDraft(ctx context.Context, id string, expectedVersion int64, actorID, requestID, ip string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var makerID, state string
	var rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT created_by::text,state,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, id).Scan(&makerID, &state, &rowVersion); err != nil {
		return err
	}
	if state != "DRAFT" || makerID != actorID || rowVersion != expectedVersion {
		return ErrBatchConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='CANCELLED',row_version=row_version+1,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, actorID, "BATCH_CANCELLED", "batch", id, map[string]string{"state": state}, map[string]string{"state": "CANCELLED"}, requestID, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) ReopenForCorrection(ctx context.Context, id string, expectedVersion int64, actorID, requestID, ip string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var makerID, state string
	var rowVersion int64
	var sourceVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT created_by::text,state,row_version,source_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, id).Scan(&makerID, &state, &rowVersion, &sourceVersion); err != nil {
		return err
	}
	if state != "REVIEW" || makerID != actorID || rowVersion != expectedVersion {
		return ErrBatchConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transaction_matches SET active=false WHERE batch_id=$1 AND active`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='ARCHIVED' WHERE batch_id=$1 AND state<>'ARCHIVED'`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET state='INVALIDATED' WHERE batch_id=$1 AND state='PENDING'`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_runs SET state='ARCHIVED' WHERE batch_id=$1 AND state<>'ARCHIVED'`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE import_files SET state='SUPERSEDED' WHERE batch_id=$1 AND state='VALID'`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='DRAFT',source_version=source_version+1,row_version=row_version+1,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, actorID, "BATCH_REOPENED", "batch", id, map[string]any{"state": state, "source_version": sourceVersion}, map[string]any{"state": "DRAFT", "source_version": sourceVersion + 1}, requestID, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func CheckOwnership(ctx context.Context, db *sql.DB, batchID, userID string, operator bool) error {
	if !operator {
		return nil
	}
	var ownerID string
	if err := db.QueryRowContext(ctx, `SELECT created_by::text FROM reconciliation_batches WHERE id=$1`, batchID).Scan(&ownerID); err != nil {
		return err
	}
	if ownerID != userID {
		return ErrForbiddenScope
	}
	return nil
}

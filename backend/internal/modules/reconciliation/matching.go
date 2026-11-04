package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"reconx/backend/internal/platform/audit"
)

var ErrRunNotReady = errors.New("batch is not ready for matching")
var ErrSubmitBlocked = errors.New("batch is not eligible for approval")

type MatchRunResult struct {
	ID            string    `json:"id"`
	BatchID       string    `json:"batch_id"`
	SourceVersion int64     `json:"source_version"`
	RuleVersion   string    `json:"rule_version"`
	State         string    `json:"state"`
	MatchedPairs  int       `json:"matched_pairs"`
	Exceptions    int       `json:"exceptions"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
}

type TransactionView struct {
	ID                 string          `json:"id"`
	BatchID            string          `json:"batch_id"`
	ImportID           string          `json:"import_id"`
	SourceType         string          `json:"source_type"`
	ReferenceID        string          `json:"reference_id"`
	ReferenceCanonical string          `json:"reference_canonical"`
	Date               time.Time       `json:"date"`
	Direction          string          `json:"direction"`
	Amount             int64           `json:"amount_rupiah"`
	Currency           string          `json:"currency"`
	SourceVersion      int64           `json:"source_version"`
	RawRow             json.RawMessage `json:"raw_row"`
}

type MatchView struct {
	ID                    string    `json:"id"`
	ReferenceID           string    `json:"reference_id"`
	InternalTransactionID string    `json:"internal_transaction_id"`
	BankTransactionID     string    `json:"bank_transaction_id"`
	InternalDate          time.Time `json:"internal_date"`
	BankDate              time.Time `json:"bank_date"`
	InternalDirection     string    `json:"internal_direction"`
	BankDirection         string    `json:"bank_direction"`
	InternalAmount        int64     `json:"internal_amount_rupiah"`
	BankAmount            int64     `json:"bank_amount_rupiah"`
	Method                string    `json:"method"`
	RuleID                string    `json:"rule_id"`
	ApprovalID            string    `json:"approval_id,omitempty"`
}

type ExceptionView struct {
	ID                    string     `json:"id"`
	BatchID               string     `json:"batch_id"`
	BatchNo               string     `json:"batch_no"`
	RunID                 string     `json:"run_id"`
	Type                  string     `json:"type"`
	State                 string     `json:"state"`
	InternalTransactionID string     `json:"internal_transaction_id,omitempty"`
	BankTransactionID     string     `json:"bank_transaction_id,omitempty"`
	ReferenceID           string     `json:"reference_id"`
	InternalAmount        *int64     `json:"internal_amount_rupiah,omitempty"`
	BankAmount            *int64     `json:"bank_amount_rupiah,omitempty"`
	Difference            *int64     `json:"difference_rupiah,omitempty"`
	InternalDate          *time.Time `json:"internal_date,omitempty"`
	BankDate              *time.Time `json:"bank_date,omitempty"`
	InternalDirection     string     `json:"internal_direction,omitempty"`
	BankDirection         string     `json:"bank_direction,omitempty"`
	AssignedTo            string     `json:"assigned_to,omitempty"`
	AssigneeName          string     `json:"assignee_name,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

func (s Service) RunMatching(ctx context.Context, batchID string, expectedVersion int64, actorID, requestID, ip string) (MatchRunResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return MatchRunResult{}, err
	}
	var makerID, state string
	var sourceVersion, rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT created_by::text,state,source_version,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&makerID, &state, &sourceVersion, &rowVersion); err != nil {
		_ = tx.Rollback()
		return MatchRunResult{}, err
	}
	if state != "READY" || rowVersion != expectedVersion || makerID != actorID {
		_ = tx.Rollback()
		return MatchRunResult{}, ErrRunNotReady
	}
	var sources int
	if err := tx.QueryRowContext(ctx, `SELECT count(DISTINCT source_type) FROM import_files WHERE batch_id=$1 AND source_version=$2 AND state='VALID'`, batchID, sourceVersion).Scan(&sources); err != nil {
		_ = tx.Rollback()
		return MatchRunResult{}, err
	}
	if sources != 2 {
		_ = tx.Rollback()
		return MatchRunResult{}, ErrRunNotReady
	}
	var run MatchRunResult
	err = tx.QueryRowContext(ctx, `INSERT INTO reconciliation_runs(batch_id,source_version,rule_version,state) VALUES ($1,$2,'M01-v1','PROCESSING') RETURNING id::text,batch_id::text,source_version,rule_version,state,started_at`, batchID, sourceVersion).
		Scan(&run.ID, &run.BatchID, &run.SourceVersion, &run.RuleVersion, &run.State, &run.StartedAt)
	if err != nil {
		_ = tx.Rollback()
		return MatchRunResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='PROCESSING',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		_ = tx.Rollback()
		return MatchRunResult{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "MATCH_RUN_STARTED", "run", run.ID, nil, map[string]any{"batch_id": batchID, "source_version": sourceVersion, "rule_version": run.RuleVersion}, requestID, ip); err != nil {
		_ = tx.Rollback()
		return MatchRunResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return MatchRunResult{}, err
	}

	internalRows, err := s.sourceTransactions(ctx, batchID, sourceVersion, "INTERNAL")
	if err != nil {
		return MatchRunResult{}, s.failRun(ctx, run, batchID, sourceVersion, actorID, requestID, ip, err)
	}
	bankRows, err := s.sourceTransactions(ctx, batchID, sourceVersion, "BANK")
	if err != nil {
		return MatchRunResult{}, s.failRun(ctx, run, batchID, sourceVersion, actorID, requestID, ip, err)
	}
	matched, exceptions := classify(internalRows, bankRows)

	finishTx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return MatchRunResult{}, s.failRun(ctx, run, batchID, sourceVersion, actorID, requestID, ip, err)
	}
	defer finishTx.Rollback()
	failFinish := func(cause error) (MatchRunResult, error) {
		_ = finishTx.Rollback()
		return MatchRunResult{}, s.failRun(ctx, run, batchID, sourceVersion, actorID, requestID, ip, cause)
	}
	var currentState string
	var currentVersion int64
	if err := finishTx.QueryRowContext(ctx, `SELECT state,source_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&currentState, &currentVersion); err != nil {
		return failFinish(err)
	}
	if currentState != "PROCESSING" || currentVersion != sourceVersion {
		return failFinish(ErrBatchConflict)
	}
	for _, pair := range matched {
		if _, err := finishTx.ExecContext(ctx, `INSERT INTO transaction_matches(batch_id,run_id,internal_transaction_id,bank_transaction_id,method)
			VALUES ($1,$2,$3,$4,'AUTO_MATCHED')`, batchID, run.ID, pair.internal.ID, pair.bank.ID); err != nil {
			return failFinish(err)
		}
	}
	for _, item := range exceptions {
		if _, err := finishTx.ExecContext(ctx, `INSERT INTO reconciliation_exceptions(batch_id,run_id,type,internal_transaction_id,bank_transaction_id)
			VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid)`, batchID, run.ID, item.kind, item.internal.ID, item.bank.ID); err != nil {
			return failFinish(err)
		}
	}
	ended := time.Now().UTC()
	if _, err := finishTx.ExecContext(ctx, `UPDATE reconciliation_runs SET state='SUCCEEDED',ended_at=$2 WHERE id=$1`, run.ID, ended); err != nil {
		return failFinish(err)
	}
	if _, err := finishTx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='REVIEW',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return failFinish(err)
	}
	if _, err := finishTx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id)
		SELECT created_by,'MATCHING_COMPLETE','Rekonsiliasi selesai',$1,'batch',$2::uuid::text FROM reconciliation_batches WHERE id=$2::uuid`, fmt.Sprintf("%s selesai: %d pasangan cocok dan %d exception.", run.RuleVersion, len(matched), len(exceptions)), batchID); err != nil {
		return failFinish(err)
	}
	if err := audit.Append(ctx, finishTx, actorID, "MATCH_RUN_COMPLETED", "run", run.ID, nil, map[string]any{"matched_pairs": len(matched), "exceptions": len(exceptions), "source_version": sourceVersion}, requestID, ip); err != nil {
		return failFinish(err)
	}
	if err := finishTx.Commit(); err != nil {
		return MatchRunResult{}, s.failRun(ctx, run, batchID, sourceVersion, actorID, requestID, ip, err)
	}
	run.State, run.MatchedPairs, run.Exceptions, run.EndedAt = "SUCCEEDED", len(matched), len(exceptions), ended
	return run, nil
}

type identifiedPair struct {
	internal TransactionView
	bank     TransactionView
}

type identifiedException struct {
	kind     string
	internal TransactionView
	bank     TransactionView
}

func classify(internalRows, bankRows []TransactionView) ([]identifiedPair, []identifiedException) {
	internal := make(map[string]TransactionView, len(internalRows))
	bank := make(map[string]TransactionView, len(bankRows))
	keys := make(map[string]struct{}, len(internalRows)+len(bankRows))
	for _, row := range internalRows {
		internal[row.ReferenceCanonical] = row
		keys[row.ReferenceCanonical] = struct{}{}
	}
	for _, row := range bankRows {
		bank[row.ReferenceCanonical] = row
		keys[row.ReferenceCanonical] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	matches := make([]identifiedPair, 0)
	exceptions := make([]identifiedException, 0)
	for _, key := range ordered {
		internalRow, hasInternal := internal[key]
		bankRow, hasBank := bank[key]
		if !hasInternal {
			exceptions = append(exceptions, identifiedException{kind: "BANK_ONLY", bank: bankRow})
			continue
		}
		if !hasBank {
			exceptions = append(exceptions, identifiedException{kind: "INTERNAL_ONLY", internal: internalRow})
			continue
		}
		switch {
		case internalRow.Amount != bankRow.Amount:
			exceptions = append(exceptions, identifiedException{kind: "AMOUNT_MISMATCH", internal: internalRow, bank: bankRow})
		case internalRow.Direction != bankRow.Direction:
			exceptions = append(exceptions, identifiedException{kind: "DIRECTION_MISMATCH", internal: internalRow, bank: bankRow})
		case !sameCalendarDate(internalRow.Date, bankRow.Date):
			exceptions = append(exceptions, identifiedException{kind: "DATE_VARIANCE", internal: internalRow, bank: bankRow})
		default:
			matches = append(matches, identifiedPair{internal: internalRow, bank: bankRow})
		}
	}
	return matches, exceptions
}

func sameCalendarDate(left, right time.Time) bool {
	return left.Format("2006-01-02") == right.Format("2006-01-02")
}

func (s Service) sourceTransactions(ctx context.Context, batchID string, sourceVersion int64, source string) ([]TransactionView, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row
		FROM transactions WHERE batch_id=$1 AND source_version=$2 AND source_type=$3 ORDER BY reference_canonical,id`, batchID, sourceVersion, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TransactionView, 0)
	for rows.Next() {
		var item TransactionView
		if err := rows.Scan(&item.ID, &item.BatchID, &item.ImportID, &item.SourceType, &item.ReferenceID, &item.ReferenceCanonical, &item.Date, &item.Direction, &item.Amount, &item.Currency, &item.SourceVersion, &item.RawRow); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Service) failRun(ctx context.Context, run MatchRunResult, batchID string, sourceVersion int64, actorID, requestID, ip string, cause error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w; additionally failed to record failed run: %v", cause, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_runs SET state='FAILED',ended_at=now(),error=$2 WHERE id=$1 AND state='PROCESSING'`, run.ID, cause.Error()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='FAILED',row_version=row_version+1,updated_at=now() WHERE id=$1 AND state='PROCESSING' AND source_version=$2`, batchID, sourceVersion); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, actorID, "MATCH_RUN_FAILED", "run", run.ID, nil, map[string]string{"error": cause.Error()}, requestID, ip); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return cause
}

func (s Service) LatestRun(ctx context.Context, batchID string) (MatchRunResult, error) {
	var result MatchRunResult
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,batch_id::text,source_version,rule_version,state,started_at,COALESCE(ended_at,started_at) FROM reconciliation_runs WHERE batch_id=$1 ORDER BY started_at DESC LIMIT 1`, batchID).
		Scan(&result.ID, &result.BatchID, &result.SourceVersion, &result.RuleVersion, &result.State, &result.StartedAt, &result.EndedAt)
	if err != nil {
		return MatchRunResult{}, err
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM transaction_matches WHERE run_id=$1 AND active`, result.ID).Scan(&result.MatchedPairs)
	_ = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM reconciliation_exceptions WHERE run_id=$1 AND state<>'ARCHIVED'`, result.ID).Scan(&result.Exceptions)
	return result, nil
}

func (s Service) SubmitBatch(ctx context.Context, batchID string, expectedVersion int64, actorID, requestID, ip string) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var state, makerID string
	var sourceVersion, rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT state,created_by::text,source_version,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&state, &makerID, &sourceVersion, &rowVersion); err != nil {
		return "", err
	}
	if state != "REVIEW" || makerID != actorID || rowVersion != expectedVersion {
		return "", ErrSubmitBlocked
	}
	var runID string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM reconciliation_runs WHERE batch_id=$1 AND source_version=$2 AND state='SUCCEEDED' ORDER BY started_at DESC LIMIT 1`, batchID, sourceVersion).Scan(&runID); err != nil {
		return "", ErrSubmitBlocked
	}
	var internalCount, bankCount, matchCount, exceptionCount, pendingMatches int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE source_type='INTERNAL'),count(*) FILTER(WHERE source_type='BANK') FROM transactions WHERE batch_id=$1 AND source_version=$2`, batchID, sourceVersion).Scan(&internalCount, &bankCount); err != nil {
		return "", err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM transaction_matches WHERE batch_id=$1 AND run_id=$2 AND active`, batchID, runID).Scan(&matchCount); err != nil {
		return "", err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reconciliation_exceptions WHERE batch_id=$1 AND run_id=$2 AND state NOT IN ('RESOLVED','ARCHIVED')`, batchID, runID).Scan(&exceptionCount); err != nil {
		return "", err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM approval_requests WHERE batch_id=$1 AND state='PENDING' AND kind='MANUAL_MATCH'`, batchID).Scan(&pendingMatches); err != nil {
		return "", err
	}
	if internalCount == 0 || internalCount != bankCount || matchCount != internalCount || exceptionCount != 0 || pendingMatches != 0 {
		return "", ErrSubmitBlocked
	}
	var approvalID string
	err = tx.QueryRowContext(ctx, `INSERT INTO approval_requests(kind,batch_id,run_id,source_version,maker_id,state)
		VALUES ('BATCH_CLOSE',$1,$2,$3,$4,'PENDING') RETURNING id::text`, batchID, runID, sourceVersion, actorID).Scan(&approvalID)
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='PENDING_APPROVAL',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id)
		SELECT u.id,'APPROVAL_REQUESTED','Batch menunggu approval',$1,'approval',$2 FROM users u JOIN roles r ON r.id=u.role_id WHERE u.active AND r.code='CHECKER'`, fmt.Sprintf("Batch %s menunggu pemeriksaan independen.", batchID), approvalID); err != nil {
		return "", err
	}
	if err := audit.Append(ctx, tx, actorID, "BATCH_SUBMITTED", "batch", batchID, map[string]string{"state": state}, map[string]any{"state": "PENDING_APPROVAL", "approval_id": approvalID, "source_version": sourceVersion}, requestID, ip); err != nil {
		return "", err
	}
	return approvalID, tx.Commit()
}

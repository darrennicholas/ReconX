package reconciliation

import (
	"context"
	"strings"

	"reconx/backend/internal/platform/audit"
)

type TransactionSearch struct {
	Query      string
	BatchID    string
	Source     string
	Status     string
	OperatorID string
	Operator   bool
	Page       int
	PageSize   int
}

type TransactionResult struct {
	TransactionView
	Status string `json:"status"`
}

type DashboardSummary struct {
	TotalBatches       int                   `json:"total_batches"`
	ActiveBatches      int                   `json:"active_batches"`
	MatchedPairs       int                   `json:"matched_pairs"`
	UnmatchedRows      int                   `json:"unmatched_rows"`
	InternalCount      int                   `json:"internal_count"`
	BankCount          int                   `json:"bank_count"`
	MatchRate          float64               `json:"match_rate"`
	PendingApprovals   int                   `json:"pending_approvals"`
	OpenExceptions     int                   `json:"open_exceptions"`
	ExceptionsOlder24h int                   `json:"exceptions_older_24h"`
	DailyTrend         []DashboardTrendPoint `json:"daily_trend"`
	BatchStatuses      []DashboardCount      `json:"batch_statuses"`
	ExceptionReasons   []DashboardCount      `json:"exception_reasons"`
}

type DashboardTrendPoint struct {
	Date           string `json:"date"`
	MatchedCount   int    `json:"matched_count"`
	UnmatchedCount int    `json:"unmatched_count"`
}

type DashboardCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s Service) ListTransactions(ctx context.Context, filter TransactionSearch) ([]TransactionResult, int, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	base := ` FROM transactions t JOIN reconciliation_batches b ON b.id=t.batch_id
		WHERE t.source_version=b.source_version AND ($1='' OR t.reference_canonical ILIKE '%'||upper($1)||'%' OR b.batch_no ILIKE '%'||$1||'%')
		AND ($2='' OR t.batch_id::text=$2) AND ($3='' OR t.source_type=$3)
		AND ($4::boolean=false OR b.created_by::text=$5 OR EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.batch_id=b.id AND e.assigned_to::text=$5 AND e.state<>'ARCHIVED' AND (e.internal_transaction_id=t.id OR e.bank_transaction_id=t.id OR e.candidate_transaction_ids ? t.id::text)))
		AND ($6='' OR CASE WHEN EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id)) THEN 'MATCHED'
			WHEN EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND (e.internal_transaction_id=t.id OR e.bank_transaction_id=t.id OR e.candidate_transaction_ids ? t.id::text)) THEN 'EXCEPTION' ELSE 'UNMATCHED' END=$6)`
	args := []any{filter.Query, filter.BatchID, filter.Source, filter.Operator, filter.OperatorID, filter.Status}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT t.id::text,t.batch_id::text,t.import_file_id::text,t.source_type,t.reference_raw,t.reference_canonical,t.transaction_date,t.direction,t.amount_rupiah,t.currency,t.source_version,t.raw_row,
		CASE WHEN EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id)) THEN 'MATCHED'
		WHEN EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND (e.internal_transaction_id=t.id OR e.bank_transaction_id=t.id OR e.candidate_transaction_ids ? t.id::text)) THEN 'EXCEPTION' ELSE 'UNMATCHED' END` + base + ` ORDER BY t.transaction_date DESC,t.reference_canonical,t.id LIMIT $7 OFFSET $8`
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]TransactionResult, 0)
	for rows.Next() {
		var item TransactionResult
		if err := rows.Scan(&item.ID, &item.BatchID, &item.ImportID, &item.SourceType, &item.ReferenceID, &item.ReferenceCanonical, &item.Date, &item.Direction, &item.Amount, &item.Currency, &item.SourceVersion, &item.RawRow, &item.Status); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s Service) GetTransaction(ctx context.Context, id string, operatorID string, operator bool) (TransactionResult, error) {
	filter := ` WHERE t.id=$1 AND t.source_version=b.source_version AND ($2::boolean=false OR b.created_by::text=$3 OR EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.batch_id=b.id AND e.assigned_to::text=$3 AND e.state<>'ARCHIVED' AND (e.internal_transaction_id=t.id OR e.bank_transaction_id=t.id OR e.candidate_transaction_ids ? t.id::text)))`
	query := `SELECT t.id::text,t.batch_id::text,t.import_file_id::text,t.source_type,t.reference_raw,t.reference_canonical,t.transaction_date,t.direction,t.amount_rupiah,t.currency,t.source_version,t.raw_row,
	CASE WHEN EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id)) THEN 'MATCHED'
	WHEN EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND (e.internal_transaction_id=t.id OR e.bank_transaction_id=t.id OR e.candidate_transaction_ids ? t.id::text)) THEN 'EXCEPTION' ELSE 'UNMATCHED' END
	FROM transactions t JOIN reconciliation_batches b ON b.id=t.batch_id` + filter
	var item TransactionResult
	err := s.DB.QueryRowContext(ctx, query, id, operator, operatorID).Scan(&item.ID, &item.BatchID, &item.ImportID, &item.SourceType, &item.ReferenceID, &item.ReferenceCanonical, &item.Date, &item.Direction, &item.Amount, &item.Currency, &item.SourceVersion, &item.RawRow, &item.Status)
	return item, err
}

func (s Service) ListMatches(ctx context.Context, batchID string, sourceVersion int64) ([]MatchView, error) {
	items, _, err := s.ListMatchesPage(ctx, batchID, sourceVersion, "", "", 1, 100)
	return items, err
}

func (s Service) ListMatchesPage(ctx context.Context, batchID string, sourceVersion int64, query, method string, page, pageSize int) ([]MatchView, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	if sourceVersion == 0 {
		var err error
		err = s.DB.QueryRowContext(ctx, `SELECT source_version FROM reconciliation_batches WHERE id=$1`, batchID).Scan(&sourceVersion)
		if err != nil {
			return nil, 0, err
		}
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM transaction_matches m JOIN reconciliation_runs rr ON rr.id=m.run_id JOIN transactions i ON i.id=m.internal_transaction_id WHERE m.batch_id=$1 AND rr.source_version=$2 AND m.active AND ($3='' OR i.reference_canonical ILIKE '%'||upper($3)||'%') AND ($4='' OR m.method=$4)`, batchID, sourceVersion, query, method).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT m.id::text,i.reference_raw,i.id::text,b.id::text,i.transaction_date,b.transaction_date,i.direction,b.direction,i.amount_rupiah,b.amount_rupiah,m.method,rr.rule_version,COALESCE(m.approval_id::text,'')
		FROM transaction_matches m JOIN reconciliation_runs rr ON rr.id=m.run_id
		JOIN transactions i ON i.id=m.internal_transaction_id JOIN transactions b ON b.id=m.bank_transaction_id
		WHERE m.batch_id=$1 AND rr.source_version=$2 AND m.active AND ($3='' OR i.reference_canonical ILIKE '%'||upper($3)||'%') AND ($4='' OR m.method=$4) ORDER BY i.reference_canonical,i.id LIMIT $5 OFFSET $6`, batchID, sourceVersion, query, method, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]MatchView, 0)
	for rows.Next() {
		var item MatchView
		if err := rows.Scan(&item.ID, &item.ReferenceID, &item.InternalTransactionID, &item.BankTransactionID, &item.InternalDate, &item.BankDate, &item.InternalDirection, &item.BankDirection, &item.InternalAmount, &item.BankAmount, &item.Method, &item.RuleID, &item.ApprovalID); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s Service) RetryFailedBatch(ctx context.Context, batchID string, expectedVersion int64, actorID, requestID, ip string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state, makerID string
	var rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT state,created_by::text,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&state, &makerID, &rowVersion); err != nil {
		return err
	}
	if state != "FAILED" || makerID != actorID || rowVersion != expectedVersion {
		return ErrBatchConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='READY',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, actorID, "MATCH_RUN_RETRY", "batch", batchID, map[string]string{"state": state}, map[string]string{"state": "READY"}, requestID, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) Dashboard(ctx context.Context, operatorID string, operator bool, from, to, accountID string) (DashboardSummary, error) {
	var result DashboardSummary
	batchFilter := ` FROM reconciliation_batches b WHERE ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`
	var fromArg, toArg any
	if from != "" {
		fromArg = from
	}
	if to != "" {
		toArg = to
	}
	args := []any{fromArg, toArg, accountID, operator, operatorID}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE state NOT IN ('CLOSED','CANCELLED'))`+batchFilter, args...).Scan(&result.TotalBatches, &result.ActiveBatches); err != nil {
		return result, err
	}
	query := `SELECT count(*) FROM transaction_matches m JOIN reconciliation_batches b ON b.id=m.batch_id JOIN reconciliation_runs r ON r.id=m.run_id
	WHERE m.active AND r.state='SUCCEEDED' AND r.source_version=b.source_version AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&result.MatchedPairs); err != nil {
		return result, err
	}
	query = `SELECT count(*) FROM transactions t JOIN reconciliation_batches b ON b.id=t.batch_id WHERE t.source_version=b.source_version AND t.source_type='INTERNAL' AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&result.InternalCount); err != nil {
		return result, err
	}
	query = strings.Replace(query, "INTERNAL", "BANK", 1)
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&result.BankCount); err != nil {
		return result, err
	}
	query = `SELECT count(*) FROM transactions t JOIN reconciliation_batches b ON b.id=t.batch_id WHERE t.source_version=b.source_version AND NOT EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id)) AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&result.UnmatchedRows); err != nil {
		return result, err
	}
	if result.InternalCount > 0 {
		result.MatchRate = float64(result.MatchedPairs) * 100 / float64(result.InternalCount)
	}
	result.DailyTrend = make([]DashboardTrendPoint, 0)
	rows, err := s.DB.QueryContext(ctx, `SELECT t.transaction_date::text,
		count(*) FILTER(WHERE EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id))),
		count(*) FILTER(WHERE NOT EXISTS(SELECT 1 FROM transaction_matches m WHERE m.active AND (m.internal_transaction_id=t.id OR m.bank_transaction_id=t.id)))
		FROM transactions t JOIN reconciliation_batches b ON b.id=t.batch_id
		WHERE t.source_version=b.source_version AND t.source_type='INTERNAL' AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2)
		AND t.transaction_date >= COALESCE($1::date,CURRENT_DATE-29) AND t.transaction_date <= COALESCE($2::date,CURRENT_DATE)
		AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)
		GROUP BY t.transaction_date ORDER BY t.transaction_date`, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var point DashboardTrendPoint
		if err := rows.Scan(&point.Date, &point.MatchedCount, &point.UnmatchedCount); err != nil {
			rows.Close()
			return result, err
		}
		result.DailyTrend = append(result.DailyTrend, point)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	result.BatchStatuses = make([]DashboardCount, 0)
	rows, err = s.DB.QueryContext(ctx, `SELECT state,count(*)`+batchFilter+` GROUP BY state ORDER BY state`, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item DashboardCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return result, err
		}
		result.BatchStatuses = append(result.BatchStatuses, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	result.ExceptionReasons = make([]DashboardCount, 0)
	rows, err = s.DB.QueryContext(ctx, `SELECT e.type,count(*) FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id
		WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2)
		AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5) GROUP BY e.type ORDER BY count(*) DESC,e.type LIMIT 5`, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item DashboardCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return result, err
		}
		result.ExceptionReasons = append(result.ExceptionReasons, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	return result, nil
}

func (s Service) OpenExceptionSummary(ctx context.Context, operatorID string, operator bool, from, to, accountID string) (open, older24h int, err error) {
	var fromArg, toArg any
	if from != "" {
		fromArg = from
	}
	if to != "" {
		toArg = to
	}
	query := `SELECT count(*),count(*) FILTER(WHERE e.created_at < now()-interval '24 hours') FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`
	err = s.DB.QueryRowContext(ctx, query, fromArg, toArg, accountID, operator, operatorID).Scan(&open, &older24h)
	return
}

func (s Service) PendingApprovals(ctx context.Context, operatorID string, operator bool, from, to, accountID string) (int, error) {
	var count int
	var fromArg, toArg any
	if from != "" {
		fromArg = from
	}
	if to != "" {
		toArg = to
	}
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM approval_requests a JOIN reconciliation_batches b ON b.id=a.batch_id WHERE a.state='PENDING' AND ($1::date IS NULL OR b.period_from >= $1) AND ($2::date IS NULL OR b.period_to <= $2) AND ($3='' OR b.account_id::text=$3) AND ($4::boolean=false OR b.created_by::text=$5)`, fromArg, toArg, accountID, operator, operatorID).Scan(&count)
	return count, err
}

func (s Service) GetImportBatch(ctx context.Context, importID string) (string, error) {
	var batchID string
	err := s.DB.QueryRowContext(ctx, `SELECT batch_id::text FROM import_files WHERE id=$1`, importID).Scan(&batchID)
	return batchID, err
}

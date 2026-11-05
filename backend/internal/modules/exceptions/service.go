package exceptions

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/modules/reconciliation"
	"reconx/backend/internal/platform/audit"
)

var (
	ErrInvalidAction    = errors.New("invalid exception action")
	ErrNotEligible      = errors.New("exception is not eligible for manual match")
	ErrApprovalConflict = errors.New("approval is not applicable to this snapshot")
)

type Service struct {
	DB          *sql.DB
	StoragePath string
}

type ExceptionFilter struct {
	Query      string
	Type       string
	State      string
	BatchID    string
	AccountID  string
	AssigneeID string
	From       string
	To         string
	AgeBucket  string
	OperatorID string
	Operator   bool
	Page       int
	PageSize   int
}

type ExceptionAction struct {
	ID          string    `json:"id"`
	ExceptionID string    `json:"exception_id"`
	ActorID     string    `json:"actor_id"`
	ActorName   string    `json:"actor_name"`
	ActionType  string    `json:"action_type"`
	Note        string    `json:"note"`
	Evidence    bool      `json:"has_evidence"`
	EvidenceID  string    `json:"evidence_id,omitempty"`
	FileName    string    `json:"file_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type EvidenceFile struct {
	Path        string
	FileName    string
	ContentType string
}

type ExceptionDetail struct {
	reconciliation.ExceptionView
	Internal        *reconciliation.TransactionView  `json:"internal,omitempty"`
	Bank            *reconciliation.TransactionView  `json:"bank,omitempty"`
	Candidates      []reconciliation.TransactionView `json:"candidates,omitempty"`
	Actions         []ExceptionAction                `json:"actions"`
	CanAssignOthers bool                             `json:"can_assign_others"`
}

type ApprovalView struct {
	ID            string                          `json:"id"`
	Kind          string                          `json:"kind"`
	BatchID       string                          `json:"batch_id"`
	BatchNo       string                          `json:"batch_no"`
	ExceptionID   string                          `json:"exception_id,omitempty"`
	ExceptionType string                          `json:"exception_type,omitempty"`
	MakerID       string                          `json:"maker_id"`
	MakerName     string                          `json:"maker_name"`
	CheckerID     string                          `json:"checker_id,omitempty"`
	State         string                          `json:"state"`
	SourceVersion int64                           `json:"source_version"`
	Reason        string                          `json:"reason"`
	Comment       string                          `json:"decision_comment"`
	CreatedAt     time.Time                       `json:"created_at"`
	DecidedAt     *time.Time                      `json:"decided_at,omitempty"`
	Internal      *reconciliation.TransactionView `json:"internal,omitempty"`
	Bank          *reconciliation.TransactionView `json:"bank,omitempty"`
	InternalFile  string                          `json:"internal_file,omitempty"`
	BankFile      string                          `json:"bank_file,omitempty"`
	RuleVersion   string                          `json:"rule_version,omitempty"`
}

type ActionInput struct {
	Type       string `json:"type"`
	Note       string `json:"note"`
	AssigneeID string `json:"assignee_id"`
}

type ProposalInput struct {
	Reason string `json:"reason"`
}

func checkInvestigationAccess(actorID, makerID, assignedTo, exceptionState, batchState, action string) error {
	if actorID == "" || (actorID != makerID && actorID != assignedTo) {
		return reconciliation.ErrForbiddenScope
	}
	if batchState != "REVIEW" || exceptionState == "RESOLVED" || exceptionState == "ARCHIVED" {
		return ErrApprovalConflict
	}
	if action == "ASSIGN" && exceptionState != "OPEN" && exceptionState != "INVESTIGATING" {
		return ErrApprovalConflict
	}
	if action != "ASSIGN" && exceptionState != "OPEN" && exceptionState != "INVESTIGATING" && exceptionState != "PENDING_ACTION_APPROVAL" {
		return ErrApprovalConflict
	}
	return nil
}

func (s Service) List(ctx context.Context, filter ExceptionFilter) ([]reconciliation.ExceptionView, int, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	base := ` FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id
		LEFT JOIN users u ON u.id=e.assigned_to
		LEFT JOIN transactions i ON i.id=e.internal_transaction_id LEFT JOIN transactions k ON k.id=e.bank_transaction_id
		WHERE ($1='' OR e.type=$1) AND ($2='' OR e.state=$2) AND ($3='' OR e.batch_id::text=$3) AND ($4='' OR b.account_id::text=$4)
		AND ($5='' OR COALESCE(e.assigned_to::text,'')=$5) AND ($6='' OR e.created_at::date >= $6::date) AND ($7='' OR e.created_at::date <= $7::date)
		AND ($8='' OR e.id::text=$8 OR COALESCE(i.reference_canonical,k.reference_canonical,'') ILIKE '%'||upper($8)||'%' OR b.batch_no ILIKE '%'||$8||'%')
		AND ($9::boolean=false OR b.created_by::text=$10 OR e.assigned_to::text=$10)
		AND ($11='' OR ($11='UNDER_24H' AND e.created_at >= now()-interval '24 hours') OR ($11='24_72H' AND e.created_at < now()-interval '24 hours' AND e.created_at >= now()-interval '72 hours') OR ($11='OVER_72H' AND e.created_at < now()-interval '72 hours'))`
	args := []any{filter.Type, filter.State, filter.BatchID, filter.AccountID, filter.AssigneeID, filter.From, filter.To, filter.Query, filter.Operator, filter.OperatorID, filter.AgeBucket}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT e.id::text,e.batch_id::text,b.batch_no,e.run_id::text,e.type,e.state,COALESCE(e.internal_transaction_id::text,''),COALESCE(e.bank_transaction_id::text,''),COALESCE(i.reference_raw,k.reference_raw,''),i.amount_rupiah,k.amount_rupiah,
		CASE WHEN i.amount_rupiah IS NOT NULL AND k.amount_rupiah IS NOT NULL THEN abs(i.amount_rupiah-k.amount_rupiah) ELSE COALESCE(i.amount_rupiah,k.amount_rupiah) END,
		i.transaction_date,k.transaction_date,COALESCE(i.direction,''),COALESCE(k.direction,''),e.candidate_transaction_ids,COALESCE(e.assigned_to::text,''),COALESCE(u.full_name,''),e.created_at` + base + ` ORDER BY e.created_at DESC LIMIT $12 OFFSET $13`
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]reconciliation.ExceptionView, 0)
	for rows.Next() {
		var item reconciliation.ExceptionView
		var candidateIDs []byte
		if err := rows.Scan(&item.ID, &item.BatchID, &item.BatchNo, &item.RunID, &item.Type, &item.State, &item.InternalTransactionID, &item.BankTransactionID, &item.ReferenceID,
			&item.InternalAmount, &item.BankAmount, &item.Difference, &item.InternalDate, &item.BankDate, &item.InternalDirection, &item.BankDirection, &candidateIDs, &item.AssignedTo, &item.AssigneeName, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(candidateIDs, &item.CandidateTransactionIDs); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// Summary returns counts for active exceptions using the same scope and filters as List.
func (s Service) Summary(ctx context.Context, filter ExceptionFilter) ([]reconciliation.DashboardCount, error) {
	base := ` FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id
		LEFT JOIN transactions i ON i.id=e.internal_transaction_id LEFT JOIN transactions k ON k.id=e.bank_transaction_id
		WHERE e.state NOT IN ('RESOLVED','ARCHIVED') AND ($1='' OR e.type=$1) AND ($2='' OR e.state=$2)
		AND ($3='' OR e.batch_id::text=$3) AND ($4='' OR b.account_id::text=$4)
		AND ($5='' OR COALESCE(e.assigned_to::text,'')=$5) AND ($6='' OR e.created_at::date >= $6::date) AND ($7='' OR e.created_at::date <= $7::date)
		AND ($8='' OR e.id::text=$8 OR COALESCE(i.reference_canonical,k.reference_canonical,'') ILIKE '%'||upper($8)||'%' OR b.batch_no ILIKE '%'||$8||'%')
		AND ($9::boolean=false OR b.created_by::text=$10 OR e.assigned_to::text=$10)
		AND ($11='' OR ($11='UNDER_24H' AND e.created_at >= now()-interval '24 hours') OR ($11='24_72H' AND e.created_at < now()-interval '24 hours' AND e.created_at >= now()-interval '72 hours') OR ($11='OVER_72H' AND e.created_at < now()-interval '72 hours'))`
	rows, err := s.DB.QueryContext(ctx, `SELECT e.type,count(*)`+base+` GROUP BY e.type ORDER BY e.type`, filter.Type, filter.State, filter.BatchID, filter.AccountID, filter.AssigneeID, filter.From, filter.To, filter.Query, filter.Operator, filter.OperatorID, filter.AgeBucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]reconciliation.DashboardCount, 0)
	for rows.Next() {
		var item reconciliation.DashboardCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Service) Detail(ctx context.Context, exceptionID, operatorID string, operator bool) (ExceptionDetail, error) {
	var detail ExceptionDetail
	var ownerID string
	var candidateIDs []byte
	err := s.DB.QueryRowContext(ctx, `SELECT e.id::text,e.batch_id::text,b.batch_no,e.run_id::text,e.type,e.state,COALESCE(e.internal_transaction_id::text,''),COALESCE(e.bank_transaction_id::text,''),COALESCE(i.reference_raw,k.reference_raw,''),i.amount_rupiah,k.amount_rupiah,
		CASE WHEN i.amount_rupiah IS NOT NULL AND k.amount_rupiah IS NOT NULL THEN abs(i.amount_rupiah-k.amount_rupiah) ELSE COALESCE(i.amount_rupiah,k.amount_rupiah) END,
		i.transaction_date,k.transaction_date,COALESCE(i.direction,''),COALESCE(k.direction,''),e.candidate_transaction_ids,COALESCE(e.assigned_to::text,''),COALESCE(u.full_name,''),e.created_at,b.created_by::text
		FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id LEFT JOIN users u ON u.id=e.assigned_to
		LEFT JOIN transactions i ON i.id=e.internal_transaction_id LEFT JOIN transactions k ON k.id=e.bank_transaction_id
		WHERE e.id=$1 AND ($2::boolean=false OR b.created_by::text=$3 OR e.assigned_to::text=$3)`, exceptionID, operator, operatorID).
		Scan(&detail.ID, &detail.BatchID, &detail.BatchNo, &detail.RunID, &detail.Type, &detail.State, &detail.InternalTransactionID, &detail.BankTransactionID, &detail.ReferenceID,
			&detail.InternalAmount, &detail.BankAmount, &detail.Difference, &detail.InternalDate, &detail.BankDate, &detail.InternalDirection, &detail.BankDirection, &candidateIDs, &detail.AssignedTo, &detail.AssigneeName, &detail.CreatedAt, &ownerID)
	if err != nil {
		return ExceptionDetail{}, err
	}
	detail.CanAssignOthers = operator && ownerID == operatorID
	if err := json.Unmarshal(candidateIDs, &detail.CandidateTransactionIDs); err != nil {
		return ExceptionDetail{}, err
	}
	for _, id := range detail.CandidateTransactionIDs {
		item, err := transactionByID(ctx, s.DB, id)
		if err != nil {
			return ExceptionDetail{}, err
		}
		if item.BatchID != detail.BatchID {
			return ExceptionDetail{}, ErrApprovalConflict
		}
		detail.Candidates = append(detail.Candidates, item)
	}
	if detail.InternalTransactionID != "" {
		item, err := transactionByID(ctx, s.DB, detail.InternalTransactionID)
		if err != nil {
			return ExceptionDetail{}, err
		}
		detail.Internal = &item
	}
	if detail.BankTransactionID != "" {
		item, err := transactionByID(ctx, s.DB, detail.BankTransactionID)
		if err != nil {
			return ExceptionDetail{}, err
		}
		detail.Bank = &item
	}
	detail.Actions = make([]ExceptionAction, 0)
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id::text,a.exception_id::text,a.actor_id::text,u.full_name,a.action_type,a.note,(a.evidence_key IS NOT NULL),COALESCE(a.evidence_filename,''),a.created_at
		FROM exception_actions a JOIN users u ON u.id=a.actor_id WHERE a.exception_id=$1 ORDER BY a.created_at`, exceptionID)
	if err != nil {
		return ExceptionDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var action ExceptionAction
		if err := rows.Scan(&action.ID, &action.ExceptionID, &action.ActorID, &action.ActorName, &action.ActionType, &action.Note, &action.Evidence, &action.FileName, &action.CreatedAt); err != nil {
			return ExceptionDetail{}, err
		}
		if action.Evidence {
			action.EvidenceID = action.ID
		}
		detail.Actions = append(detail.Actions, action)
	}
	return detail, rows.Err()
}

func transactionByID(ctx context.Context, db *sql.DB, id string) (reconciliation.TransactionView, error) {
	var item reconciliation.TransactionView
	err := db.QueryRowContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row FROM transactions WHERE id=$1`, id).
		Scan(&item.ID, &item.BatchID, &item.ImportID, &item.SourceType, &item.ReferenceID, &item.ReferenceCanonical, &item.Date, &item.Direction, &item.Amount, &item.Currency, &item.SourceVersion, &item.RawRow)
	return item, err
}

func (s Service) AddAction(ctx context.Context, exceptionID string, input ActionInput, actorID, requestID, ip string) (ExceptionAction, error) {
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.Note = strings.TrimSpace(input.Note)
	if (input.Type != "ASSIGN" && input.Type != "NOTE") || (input.Type == "NOTE" && input.Note == "") || len(input.Note) > 2000 {
		return ExceptionAction{}, ErrInvalidAction
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ExceptionAction{}, err
	}
	defer tx.Rollback()
	var batchID, state, assignedTo, makerID, batchState string
	err = tx.QueryRowContext(ctx, `SELECT e.batch_id::text,e.state,COALESCE(e.assigned_to::text,''),b.created_by::text,b.state FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id WHERE e.id=$1 FOR UPDATE OF e,b`, exceptionID).
		Scan(&batchID, &state, &assignedTo, &makerID, &batchState)
	if err != nil {
		return ExceptionAction{}, err
	}
	if err := checkInvestigationAccess(actorID, makerID, assignedTo, state, batchState, input.Type); err != nil {
		return ExceptionAction{}, err
	}
	if input.Type == "ASSIGN" {
		targetID := strings.TrimSpace(input.AssigneeID)
		if targetID == "" {
			targetID = actorID
		}
		if targetID != actorID && actorID != makerID {
			return ExceptionAction{}, reconciliation.ErrForbiddenScope
		}
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT u.active FROM users u JOIN roles r ON r.id=u.role_id WHERE u.id::text=$1 AND r.code='OPERATOR' FOR SHARE OF u`, targetID).Scan(&active); errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
			return ExceptionAction{}, ErrInvalidAction
		} else if err != nil {
			return ExceptionAction{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET assigned_to=$2,state='INVESTIGATING' WHERE id=$1`, exceptionID, targetID); err != nil {
			return ExceptionAction{}, err
		}
		input.AssigneeID = targetID
		if targetID != actorID {
			if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) VALUES ($1,'EXCEPTION_ASSIGNED','Exception ditugaskan','Anda mendapat tugas pemeriksaan exception.','exception',$2)`, targetID, exceptionID); err != nil {
				return ExceptionAction{}, err
			}
		}
	} else if state == "OPEN" {
		if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='INVESTIGATING' WHERE id=$1`, exceptionID); err != nil {
			return ExceptionAction{}, err
		}
	}
	var action ExceptionAction
	err = tx.QueryRowContext(ctx, `INSERT INTO exception_actions(exception_id,actor_id,action_type,note) VALUES ($1,$2,$3,$4)
		RETURNING id::text,exception_id::text,actor_id::text,action_type,note,(evidence_key IS NOT NULL),created_at`, exceptionID, actorID, input.Type, input.Note).
		Scan(&action.ID, &action.ExceptionID, &action.ActorID, &action.ActionType, &action.Note, &action.Evidence, &action.CreatedAt)
	if err != nil {
		return ExceptionAction{}, err
	}
	afterAssigned, afterState := assignedTo, state
	if input.Type == "ASSIGN" {
		afterAssigned, afterState = input.AssigneeID, "INVESTIGATING"
	} else if state == "OPEN" {
		afterState = "INVESTIGATING"
	}
	if err := audit.Append(ctx, tx, actorID, "EXCEPTION_"+input.Type, "exception", exceptionID, map[string]string{"state": state, "assigned_to": assignedTo}, map[string]string{"state": afterState, "note": input.Note, "assigned_to": afterAssigned}, requestID, ip); err != nil {
		return ExceptionAction{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return ExceptionAction{}, err
	}
	return action, tx.Commit()
}

func (s Service) AddEvidence(ctx context.Context, exceptionID, filename, contentType string, data []byte, actorID, requestID, ip string) (ExceptionAction, error) {
	if len(data) == 0 || len(data) > 5<<20 || filename == "" {
		return ExceptionAction{}, ErrInvalidAction
	}
	ext := map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png", "text/plain": ".txt"}[contentType]
	if ext == "" {
		return ExceptionAction{}, ErrInvalidAction
	}
	storageKey, path, err := s.storeEvidence(data, ext)
	if err != nil {
		return ExceptionAction{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path)
		}
	}()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ExceptionAction{}, err
	}
	defer tx.Rollback()
	var batchID, state, assignedTo, makerID, batchState string
	if err := tx.QueryRowContext(ctx, `SELECT e.batch_id::text,e.state,COALESCE(e.assigned_to::text,''),b.created_by::text,b.state FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id WHERE e.id=$1 FOR UPDATE OF e,b`, exceptionID).
		Scan(&batchID, &state, &assignedTo, &makerID, &batchState); err != nil {
		return ExceptionAction{}, err
	}
	if err := checkInvestigationAccess(actorID, makerID, assignedTo, state, batchState, "EVIDENCE"); err != nil {
		return ExceptionAction{}, err
	}
	if state == "OPEN" {
		if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='INVESTIGATING' WHERE id=$1`, exceptionID); err != nil {
			return ExceptionAction{}, err
		}
	}
	var action ExceptionAction
	err = tx.QueryRowContext(ctx, `INSERT INTO exception_actions(exception_id,actor_id,action_type,note,evidence_key,evidence_filename,evidence_content_type)
		VALUES ($1,$2,'EVIDENCE',$3,$4,$5,$6) RETURNING id::text,exception_id::text,actor_id::text,action_type,note,true,id::text,evidence_filename,created_at`, exceptionID, actorID, filename, storageKey, filename, contentType).
		Scan(&action.ID, &action.ExceptionID, &action.ActorID, &action.ActionType, &action.Note, &action.Evidence, &action.EvidenceID, &action.FileName, &action.CreatedAt)
	if err != nil {
		return ExceptionAction{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id=$1`, actorID).Scan(&action.ActorName); err != nil {
		return ExceptionAction{}, err
	}
	if err := audit.Append(ctx, tx, actorID, "EXCEPTION_EVIDENCE_UPLOADED", "exception", exceptionID, nil, map[string]string{"file_name": filename}, requestID, ip); err != nil {
		return ExceptionAction{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return ExceptionAction{}, err
	}
	if err := tx.Commit(); err != nil {
		return ExceptionAction{}, err
	}
	cleanup = false
	return action, nil
}

func (s Service) DownloadEvidence(ctx context.Context, exceptionID, evidenceID, operatorID string, operator bool) (EvidenceFile, error) {
	var key string
	var file EvidenceFile
	err := s.DB.QueryRowContext(ctx, `SELECT a.evidence_key,a.evidence_filename,a.evidence_content_type
		FROM exception_actions a JOIN reconciliation_exceptions e ON e.id=a.exception_id JOIN reconciliation_batches b ON b.id=e.batch_id
		WHERE a.id=$1 AND a.exception_id=$2 AND a.evidence_key IS NOT NULL AND ($3::boolean=false OR b.created_by::text=$4 OR e.assigned_to::text=$4)`, evidenceID, exceptionID, operator, operatorID).
		Scan(&key, &file.FileName, &file.ContentType)
	if err != nil {
		return EvidenceFile{}, err
	}
	if filepath.Base(key) != key || key == "." || key == "" {
		return EvidenceFile{}, sql.ErrNoRows
	}
	root := s.StoragePath
	if root == "" {
		root = filepath.Join("private-data", "imports")
	}
	file.Path = filepath.Join(root, key)
	return file, nil
}

func (s Service) storeEvidence(data []byte, ext string) (string, string, error) {
	root := s.StoragePath
	if root == "" {
		root = filepath.Join("private-data", "imports")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", "", err
	}
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	key := hex.EncodeToString(random) + ext
	path := filepath.Join(root, key)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", "", err
	}
	return key, path, nil
}

func (s Service) ProposeManualMatch(ctx context.Context, exceptionID, reason, makerID, requestID, ip string) (ApprovalView, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 20 {
		return ApprovalView{}, ErrInvalidAction
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ApprovalView{}, err
	}
	defer tx.Rollback()
	var batchID, runID, state, exceptionType, batchState string
	var currentVersion int64
	var createdBy, assignedTo string
	var internalID, bankID string
	err = tx.QueryRowContext(ctx, `SELECT e.batch_id::text,e.run_id::text,e.state,e.type,COALESCE(e.internal_transaction_id::text,''),COALESCE(e.bank_transaction_id::text,''),b.source_version,b.created_by::text,COALESCE(e.assigned_to::text,''),b.state
		FROM reconciliation_exceptions e JOIN reconciliation_batches b ON b.id=e.batch_id WHERE e.id=$1 FOR UPDATE OF e,b`, exceptionID).
		Scan(&batchID, &runID, &state, &exceptionType, &internalID, &bankID, &currentVersion, &createdBy, &assignedTo, &batchState)
	if err != nil {
		return ApprovalView{}, err
	}
	if err := checkInvestigationAccess(makerID, createdBy, assignedTo, state, batchState, "PROPOSE"); err != nil {
		return ApprovalView{}, err
	}
	if exceptionType != "DATE_VARIANCE" || (state != "OPEN" && state != "INVESTIGATING") || internalID == "" || bankID == "" {
		return ApprovalView{}, ErrNotEligible
	}
	var internal reconciliation.TransactionView
	var bank reconciliation.TransactionView
	if err := scanTransaction(tx.QueryRowContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row FROM transactions WHERE id=$1`, internalID), &internal); err != nil {
		return ApprovalView{}, err
	}
	if err := scanTransaction(tx.QueryRowContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row FROM transactions WHERE id=$1`, bankID), &bank); err != nil {
		return ApprovalView{}, err
	}
	if internal.SourceType != "INTERNAL" || bank.SourceType != "BANK" || internal.ReferenceCanonical != bank.ReferenceCanonical || internal.Amount != bank.Amount || internal.Direction != bank.Direction || internal.Currency != bank.Currency || internal.SourceVersion != currentVersion || bank.SourceVersion != currentVersion || !withinCalendarDays(internal.Date, bank.Date, 3) {
		return ApprovalView{}, ErrNotEligible
	}
	var alreadyMatched bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM transaction_matches WHERE active AND (internal_transaction_id=$1 OR bank_transaction_id=$1 OR internal_transaction_id=$2 OR bank_transaction_id=$2))`, internalID, bankID).Scan(&alreadyMatched); err != nil {
		return ApprovalView{}, err
	}
	if alreadyMatched {
		return ApprovalView{}, ErrNotEligible
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM approval_requests WHERE exception_id=$1 AND state='PENDING')`, exceptionID).Scan(&pending); err != nil {
		return ApprovalView{}, err
	}
	if pending {
		return ApprovalView{}, ErrNotEligible
	}
	payload, _ := json.Marshal(map[string]string{"internal_transaction_id": internalID, "bank_transaction_id": bankID})
	var approvalID string
	err = tx.QueryRowContext(ctx, `INSERT INTO approval_requests(kind,batch_id,exception_id,run_id,source_version,maker_id,state,match_proposal_payload,reason)
		VALUES ('MANUAL_MATCH',$1,$2,$3,$4,$5,'PENDING',$6::jsonb,$7) RETURNING id::text`, batchID, exceptionID, runID, currentVersion, makerID, string(payload), reason).Scan(&approvalID)
	if err != nil {
		return ApprovalView{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='PENDING_ACTION_APPROVAL' WHERE id=$1`, exceptionID); err != nil {
		return ApprovalView{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) SELECT u.id,'APPROVAL_REQUESTED','Manual match menunggu Checker',$1,'approval',$2 FROM users u JOIN roles r ON r.id=u.role_id WHERE u.active AND r.code='CHECKER'`, fmt.Sprintf("Usulan match %s memerlukan persetujuan.", internal.ReferenceID), approvalID); err != nil {
		return ApprovalView{}, err
	}
	if err := audit.Append(ctx, tx, makerID, "MANUAL_MATCH_PROPOSED", "approval", approvalID, nil, map[string]any{"exception_id": exceptionID, "source_version": currentVersion}, requestID, ip); err != nil {
		return ApprovalView{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return ApprovalView{}, err
	}
	if err := tx.Commit(); err != nil {
		return ApprovalView{}, err
	}
	return s.GetApproval(ctx, approvalID)
}

func withinCalendarDays(left, right time.Time, maxDays int) bool {
	leftDate := time.Date(left.Year(), left.Month(), left.Day(), 0, 0, 0, 0, time.UTC)
	rightDate := time.Date(right.Year(), right.Month(), right.Day(), 0, 0, 0, 0, time.UTC)
	delta := leftDate.Sub(rightDate)
	if delta < 0 {
		delta = -delta
	}
	return delta <= time.Duration(maxDays)*24*time.Hour
}

func scanTransaction(row *sql.Row, item *reconciliation.TransactionView) error {
	return row.Scan(&item.ID, &item.BatchID, &item.ImportID, &item.SourceType, &item.ReferenceID, &item.ReferenceCanonical, &item.Date, &item.Direction, &item.Amount, &item.Currency, &item.SourceVersion, &item.RawRow)
}

func (s Service) ListApprovals(ctx context.Context, state, kind string) ([]ApprovalView, error) {
	items, _, err := s.ListApprovalsPage(ctx, state, kind, "", 1, 100)
	return items, err
}

func (s Service) ListApprovalsPage(ctx context.Context, state, kind, query string, page, pageSize int) ([]ApprovalView, int, error) {
	return s.listApprovalsPage(ctx, state, kind, query, "", page, pageSize)
}

func (s Service) listApprovalsPage(ctx context.Context, state, kind, query, approvalID string, page, pageSize int) ([]ApprovalView, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM approval_requests a JOIN reconciliation_batches b ON b.id=a.batch_id JOIN users m ON m.id=a.maker_id WHERE ($1='' OR a.state=$1 OR ($1='HISTORY' AND a.state<>'PENDING')) AND ($2='' OR a.kind=$2) AND ($3='' OR b.batch_no ILIKE '%'||$3||'%' OR m.full_name ILIKE '%'||$3||'%' OR a.id::text ILIKE '%'||$3||'%') AND ($4='' OR a.id::text=$4)`, state, kind, query, approvalID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id::text,a.kind,a.batch_id::text,b.batch_no,COALESCE(a.exception_id::text,''),COALESCE(e.type,''),a.maker_id::text,m.full_name,COALESCE(a.checker_id::text,''),a.state,a.source_version,a.reason,a.decision_comment,a.created_at,a.decided_at,
		COALESCE(a.match_proposal_payload->>'internal_transaction_id',''),COALESCE(a.match_proposal_payload->>'bank_transaction_id','')
		FROM approval_requests a JOIN reconciliation_batches b ON b.id=a.batch_id JOIN users m ON m.id=a.maker_id LEFT JOIN reconciliation_exceptions e ON e.id=a.exception_id
		WHERE ($1='' OR a.state=$1 OR ($1='HISTORY' AND a.state<>'PENDING')) AND ($2='' OR a.kind=$2) AND ($3='' OR b.batch_no ILIKE '%'||$3||'%' OR m.full_name ILIKE '%'||$3||'%' OR a.id::text ILIKE '%'||$3||'%') AND ($6='' OR a.id::text=$6) ORDER BY CASE WHEN a.state='PENDING' THEN 0 ELSE 1 END,a.created_at DESC LIMIT $4 OFFSET $5`, state, kind, query, pageSize, (page-1)*pageSize, approvalID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]ApprovalView, 0)
	for rows.Next() {
		var item ApprovalView
		var internalID, bankID string
		if err := rows.Scan(&item.ID, &item.Kind, &item.BatchID, &item.BatchNo, &item.ExceptionID, &item.ExceptionType, &item.MakerID, &item.MakerName, &item.CheckerID, &item.State, &item.SourceVersion, &item.Reason, &item.Comment, &item.CreatedAt, &item.DecidedAt, &internalID, &bankID); err != nil {
			return nil, 0, err
		}
		if internalID != "" {
			var itemTx reconciliation.TransactionView
			if err := scanTransaction(s.DB.QueryRowContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row FROM transactions WHERE id=$1`, internalID), &itemTx); err != nil {
				return nil, 0, err
			}
			item.Internal = &itemTx
		}
		if bankID != "" {
			var itemTx reconciliation.TransactionView
			if err := scanTransaction(s.DB.QueryRowContext(ctx, `SELECT id::text,batch_id::text,import_file_id::text,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row FROM transactions WHERE id=$1`, bankID), &itemTx); err != nil {
				return nil, 0, err
			}
			item.Bank = &itemTx
		}
		if item.Kind == "MANUAL_MATCH" && item.Internal != nil && item.Bank != nil {
			if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE((SELECT original_name FROM import_files WHERE id=$1),''), COALESCE((SELECT original_name FROM import_files WHERE id=$2),''), COALESCE((SELECT rule_version FROM reconciliation_runs WHERE id=a.run_id),'') FROM approval_requests a WHERE a.id=$3`, item.Internal.ImportID, item.Bank.ImportID, item.ID).Scan(&item.InternalFile, &item.BankFile, &item.RuleVersion); err != nil {
				return nil, 0, err
			}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s Service) GetApproval(ctx context.Context, approvalID string) (ApprovalView, error) {
	items, _, err := s.listApprovalsPage(ctx, "", "", "", approvalID, 1, 1)
	if err != nil {
		return ApprovalView{}, err
	}
	for _, item := range items {
		if item.ID == approvalID {
			return item, nil
		}
	}
	return ApprovalView{}, sql.ErrNoRows
}

func (s Service) DecideApproval(ctx context.Context, approvalID, decision, comment, checkerID, requestID, ip string) error {
	decision = strings.ToUpper(strings.TrimSpace(decision))
	comment = strings.TrimSpace(comment)
	if decision != "APPROVE" && decision != "REJECT" || (decision == "REJECT" && comment == "") {
		return ErrInvalidAction
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var item ApprovalView
	var runID, internalID, bankID string
	err = tx.QueryRowContext(ctx, `SELECT id::text,kind,batch_id::text,COALESCE(exception_id::text,''),COALESCE(run_id::text,''),source_version,maker_id::text,state,COALESCE(match_proposal_payload->>'internal_transaction_id',''),COALESCE(match_proposal_payload->>'bank_transaction_id','')
		FROM approval_requests WHERE id=$1 FOR UPDATE`, approvalID).
		Scan(&item.ID, &item.Kind, &item.BatchID, &item.ExceptionID, &runID, &item.SourceVersion, &item.MakerID, &item.State, &internalID, &bankID)
	if err != nil {
		return err
	}
	if item.State != "PENDING" || item.MakerID == checkerID {
		return ErrApprovalConflict
	}
	var batchState string
	var currentVersion int64
	var rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT state,source_version,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, item.BatchID).Scan(&batchState, &currentVersion, &rowVersion); err != nil {
		return err
	}
	if currentVersion != item.SourceVersion {
		return ErrApprovalConflict
	}
	approved := decision == "APPROVE"
	newState := "REJECTED"
	if approved {
		newState = "APPROVED"
	}
	if item.Kind == "MANUAL_MATCH" {
		var exceptionState string
		if err := tx.QueryRowContext(ctx, `SELECT state FROM reconciliation_exceptions WHERE id=$1 FOR UPDATE`, item.ExceptionID).Scan(&exceptionState); err != nil {
			return err
		}
		if batchState != "REVIEW" || exceptionState != "PENDING_ACTION_APPROVAL" || runID == "" {
			return ErrApprovalConflict
		}
		if approved {
			var collision bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM transaction_matches WHERE active AND (internal_transaction_id=$1 OR bank_transaction_id=$1 OR internal_transaction_id=$2 OR bank_transaction_id=$2))`, internalID, bankID).Scan(&collision); err != nil {
				return err
			}
			if collision {
				return ErrApprovalConflict
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO transaction_matches(batch_id,run_id,internal_transaction_id,bank_transaction_id,method,approval_id) VALUES ($1,$2,$3,$4,'MANUAL_MATCHED',$5)`, item.BatchID, runID, internalID, bankID, approvalID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='RESOLVED',resolved_at=now() WHERE id=$1`, item.ExceptionID); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='INVESTIGATING' WHERE id=$1`, item.ExceptionID); err != nil {
				return err
			}
		}
	} else if item.Kind == "BATCH_CLOSE" {
		if batchState != "PENDING_APPROVAL" || runID == "" {
			return ErrApprovalConflict
		}
		if approved {
			if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='APPROVED',approved_by=$2,row_version=row_version+1,updated_at=now() WHERE id=$1`, item.BatchID, checkerID); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='REVIEW',row_version=row_version+1,updated_at=now() WHERE id=$1`, item.BatchID); err != nil {
				return err
			}
		}
	} else {
		return ErrApprovalConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET state=$2,checker_id=$3,decision_comment=$4,decided_at=now() WHERE id=$1`, approvalID, newState, checkerID, comment); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) VALUES ($1,$2,$3,$4,'batch',$5)`, item.MakerID, "APPROVAL_"+newState, "Keputusan approval tersedia", comment, item.BatchID); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, checkerID, "APPROVAL_"+newState, "approval", approvalID, map[string]string{"state": "PENDING"}, map[string]any{"state": newState, "comment": comment}, requestID, ip); err != nil {
		return err
	}
	if item.Kind == "MANUAL_MATCH" {
		if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET row_version=row_version+1,updated_at=now() WHERE id=$1`, item.BatchID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s Service) CloseApprovedBatch(ctx context.Context, batchID string, expectedVersion int64, checkerID, requestID, ip string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state, makerID, approvedBy string
	var sourceVersion, rowVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT state,created_by::text,COALESCE(approved_by::text,''),source_version,row_version FROM reconciliation_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&state, &makerID, &approvedBy, &sourceVersion, &rowVersion); err != nil {
		return err
	}
	if state != "APPROVED" || approvedBy == "" || makerID == checkerID || rowVersion != expectedVersion {
		return reconciliation.ErrBatchConflict
	}
	var validSnapshot bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM approval_requests a JOIN reconciliation_runs r ON r.id=a.run_id WHERE a.batch_id=$1 AND a.kind='BATCH_CLOSE' AND a.state='APPROVED' AND a.checker_id=$2 AND a.source_version=$3 AND r.source_version=$3 AND r.state='SUCCEEDED' AND NOT EXISTS(SELECT 1 FROM reconciliation_exceptions e WHERE e.batch_id=$1 AND e.run_id=r.id AND e.state NOT IN ('RESOLVED','ARCHIVED')) AND NOT EXISTS(SELECT 1 FROM approval_requests p WHERE p.batch_id=$1 AND p.kind='MANUAL_MATCH' AND p.state='PENDING'))`, batchID, approvedBy, sourceVersion).Scan(&validSnapshot)
	if err != nil {
		return err
	}
	if !validSnapshot {
		return reconciliation.ErrBatchConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reconciliation_batches SET state='CLOSED',row_version=row_version+1,updated_at=now() WHERE id=$1`, batchID); err != nil {
		return err
	}
	if err := audit.Append(ctx, tx, checkerID, "BATCH_CLOSED", "batch", batchID, map[string]string{"state": "APPROVED"}, map[string]any{"state": "CLOSED", "source_version": sourceVersion}, requestID, ip); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,target_entity,target_entity_id) VALUES ($1,'BATCH_CLOSED','Batch ditutup','Snapshot rekonsiliasi telah dikunci.','batch',$2)`, makerID, batchID)
	return tx.Commit()
}

func (s Service) ApprovePermission(ctx context.Context, approvalID string) (identity.Permission, error) {
	var kind string
	err := s.DB.QueryRowContext(ctx, `SELECT kind FROM approval_requests WHERE id=$1`, approvalID).Scan(&kind)
	if err != nil {
		return "", err
	}
	if kind == "MANUAL_MATCH" {
		return identity.PermissionMatchesApprove, nil
	}
	if kind == "BATCH_CLOSE" {
		return identity.PermissionBatchesApprove, nil
	}
	return "", ErrApprovalConflict
}

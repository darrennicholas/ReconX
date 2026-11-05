package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/platform/migrate"
)

// This suite creates and drops a uniquely named schema inside an explicitly supplied test database.
func TestRBACAndAssignedExceptionIntegration(t *testing.T) {
	dsn := os.Getenv("RECONX_RBAC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set RECONX_RBAC_TEST_DATABASE_URL to a dedicated PostgreSQL database ending in _test")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.Database, "_test") {
		t.Fatal("RBAC integration tests require a dedicated database whose name ends in _test")
	}
	adminDB := stdlib.OpenDB(*cfg)
	defer adminDB.Close()
	ctx := context.Background()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("rbac_it_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := adminDB.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	}()
	cfg.RuntimeParams["search_path"] = schema + ",public"
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	db.SetMaxOpenConns(8)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IMPORT_STORAGE_DIR", t.TempDir())
	hash, err := bcrypt.GenerateFromPassword([]byte("integration-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := func(email, role string) string {
		t.Helper()
		var id string
		err := db.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash,role_id) SELECT $1,$2,$3,id FROM roles WHERE code=$4 RETURNING id::text`, email, email, string(hash), role).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	adminID := user("admin-rbac@example.test", "ADMIN")
	ownerID := user("owner-rbac@example.test", "OPERATOR")
	assigneeID := user("assignee-rbac@example.test", "OPERATOR")
	outsiderID := user("outsider-rbac@example.test", "OPERATOR")
	checkerID := user("checker-rbac@example.test", "CHECKER")
	auditorID := user("auditor-rbac@example.test", "AUDITOR")
	_ = auditorID
	var accountID, batchID, runID, internalImport, bankImport, internalID, bankID, unrelatedID, exceptionID, approvalID string
	queryID := func(dest *string, q string, args ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, q, args...).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	queryID(&accountID, `INSERT INTO bank_accounts(account_code,bank_display_name,masked_account_no,account_holder_name,created_by) VALUES('TEST','Test','****1234','Test',$1) RETURNING id::text`, adminID)
	queryID(&batchID, `INSERT INTO reconciliation_batches(batch_no,account_id,period_from,period_to,state,created_by) VALUES('TEST-BATCH',$1,'2026-01-01','2026-01-31','REVIEW',$2) RETURNING id::text`, accountID, ownerID)
	queryID(&runID, `INSERT INTO reconciliation_runs(batch_id,source_version,state) VALUES($1,1,'SUCCEEDED') RETURNING id::text`, batchID)
	queryID(&internalImport, `INSERT INTO import_files(batch_id,source_type,source_version,sha256,original_name,storage_key,state,imported_by) VALUES($1,'INTERNAL',1,decode('01','hex'),'internal.csv','internal','VALID',$2) RETURNING id::text`, batchID, ownerID)
	queryID(&bankImport, `INSERT INTO import_files(batch_id,source_type,source_version,sha256,original_name,storage_key,state,imported_by) VALUES($1,'BANK',1,decode('02','hex'),'bank.csv','bank','VALID',$2) RETURNING id::text`, batchID, ownerID)
	transaction := func(importID, source, ref, date string) string {
		var id string
		queryID(&id, `INSERT INTO transactions(batch_id,import_file_id,source_type,reference_raw,reference_canonical,transaction_date,direction,amount_rupiah,currency,source_version,raw_row) VALUES($1,$2,$3,$4,$4,$5,'DEBIT',100,'IDR',1,'{}') RETURNING id::text`, batchID, importID, source, ref, date)
		return id
	}
	internalID = transaction(internalImport, "INTERNAL", "REF1", "2026-01-01")
	bankID = transaction(bankImport, "BANK", "REF1", "2026-01-02")
	unrelatedID = transaction(internalImport, "INTERNAL", "REF2", "2026-01-03")
	queryID(&exceptionID, `INSERT INTO reconciliation_exceptions(batch_id,run_id,type,state,internal_transaction_id,bank_transaction_id) VALUES($1,$2,'DATE_VARIANCE','OPEN',$3,$4) RETURNING id::text`, batchID, runID, internalID, bankID)
	queryID(&approvalID, `INSERT INTO approval_requests(kind,batch_id,run_id,source_version,maker_id,state) VALUES('BATCH_CLOSE',$1,$2,1,$3,'PENDING') RETURNING id::text`, batchID, runID, checkerID)
	auth := func(email string) identity.Credentials {
		t.Helper()
		c, err := (identity.Service{DB: db}).Login(ctx, email, "integration-password", "rbac-test", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	owner, assignee, outsider, checker, auditor, admin := auth("owner-rbac@example.test"), auth("assignee-rbac@example.test"), auth("outsider-rbac@example.test"), auth("checker-rbac@example.test"), auth("auditor-rbac@example.test"), auth("admin-rbac@example.test")
	handler := New(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	sequence := 0
	request := func(method, path, body, contentType string, c identity.Credentials) *httptest.ResponseRecorder {
		t.Helper()
		sequence++
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: c.SessionToken})
		req.AddCookie(&http.Cookie{Name: identity.CSRFCookieName, Value: c.CSRFToken})
		req.Header.Set("X-CSRF-Token", c.CSRFToken)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("rbac-test-%08d", sequence))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	assertStatus := func(rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	assertStatus(request("GET", "/api/v1/users", "", "", owner), http.StatusForbidden)
	assertStatus(request("POST", "/api/v1/batches", "{}", "application/json", auditor), http.StatusForbidden)
	assertStatus(request("POST", "/api/v1/approvals/"+approvalID+"/approve", "{}", "application/json", admin), http.StatusForbidden)
	assertStatus(request("POST", "/api/v1/approvals/"+approvalID+"/approve", "{}", "application/json", checker), http.StatusForbidden)
	var state string
	if err := db.QueryRowContext(ctx, `SELECT state FROM approval_requests WHERE id=$1`, approvalID).Scan(&state); err != nil || state != "PENDING" {
		t.Fatalf("self approval changed state: %s, %v", state, err)
	}
	path := "/api/v1/exceptions/" + exceptionID + "/actions"
	assertStatus(request("POST", path, `{"type":"ASSIGN"}`, "application/json", outsider), http.StatusForbidden)
	assertStatus(request("POST", path, `{"type":"NOTE","note":"unauthorized"}`, "application/json", outsider), http.StatusForbidden)
	assertStatus(request("POST", "/api/v1/exceptions/"+exceptionID+"/manual-match-proposals", `{"reason":"Bukti transaksi menunjukkan tanggal berbeda"}`, "application/json", outsider), http.StatusForbidden)
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	file, err := writer.CreateFormFile("file", "evidence.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("evidence for test"))
	_ = writer.Close()
	assertStatus(request("POST", "/api/v1/exceptions/"+exceptionID+"/evidence", multipartBody.String(), writer.FormDataContentType(), outsider), http.StatusForbidden)
	assertStatus(request("POST", path, fmt.Sprintf(`{"type":"ASSIGN","assignee_id":%q}`, checkerID), "application/json", owner), http.StatusUnprocessableEntity)
	assertStatus(request("POST", path, fmt.Sprintf(`{"type":"ASSIGN","assignee_id":%q}`, assigneeID), "application/json", owner), http.StatusCreated)
	assertStatus(request("GET", "/api/v1/exception-operators", "", "", owner), http.StatusOK)
	assertStatus(request("GET", "/api/v1/exception-operators", "", "", auditor), http.StatusForbidden)
	assertStatus(request("GET", "/api/v1/exceptions/"+exceptionID, "", "", assignee), http.StatusOK)
	assertStatus(request("GET", "/api/v1/batches/"+batchID, "", "", assignee), http.StatusOK)
	assertStatus(request("GET", "/api/v1/batches/"+batchID+"/matches", "", "", assignee), http.StatusForbidden)
	assertStatus(request("GET", "/api/v1/transactions/"+internalID, "", "", assignee), http.StatusOK)
	assertStatus(request("GET", "/api/v1/transactions/"+bankID, "", "", assignee), http.StatusOK)
	assertStatus(request("GET", "/api/v1/transactions/"+unrelatedID, "", "", assignee), http.StatusNotFound)
	list := request("GET", "/api/v1/transactions?batch_id="+batchID, "", "", assignee)
	assertStatus(list, http.StatusOK)
	var transactionList struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &transactionList); err != nil {
		t.Fatal(err)
	}
	if len(transactionList.Data) != 2 {
		t.Fatalf("assignee should see only two related transactions, got %d", len(transactionList.Data))
	}
	for _, item := range transactionList.Data {
		if item.ID != internalID && item.ID != bankID {
			t.Fatalf("unrelated transaction visible: %s", item.ID)
		}
	}
	candidateID := transaction(internalImport, "INTERNAL", "REF1", "2026-01-01")
	if _, err := db.ExecContext(ctx, `UPDATE reconciliation_exceptions SET candidate_transaction_ids=jsonb_build_array($2::text,$3::text,$4::text) WHERE id=$1`, exceptionID, internalID, bankID, candidateID); err != nil {
		t.Fatal(err)
	}
	assertStatus(request("GET", "/api/v1/transactions/"+candidateID, "", "", assignee), http.StatusOK)
	assertStatus(request("GET", "/api/v1/transactions/"+candidateID, "", "", outsider), http.StatusNotFound)
	detail := request("GET", "/api/v1/exceptions/"+exceptionID, "", "", assignee)
	assertStatus(detail, http.StatusOK)
	if !strings.Contains(detail.Body.String(), candidateID) {
		t.Fatal("assigned operator cannot see candidate details")
	}
	for _, reportType := range []string{"batches", "transactions", "exceptions"} {
		report := request("GET", "/api/v1/reports/reconciliations/export?type="+reportType, "", "", owner)
		assertStatus(report, http.StatusOK)
		if !strings.Contains(report.Header().Get("Content-Type"), "text/csv") {
			t.Fatalf("expected CSV for %s", reportType)
		}
		want := map[string]string{"batches": "BATCH,", "transactions": "TRANSACTION,", "exceptions": "EXCEPTION_ACTION,"}[reportType]
		if !strings.Contains(report.Body.String(), want) {
			t.Fatalf("missing %s in %s export: %s", want, reportType, report.Body.String())
		}
	}
	outsiderReport := request("GET", "/api/v1/reports/reconciliations/export?type=transactions", "", "", outsider)
	assertStatus(outsiderReport, http.StatusOK)
	if strings.Contains(outsiderReport.Body.String(), "REF1") {
		t.Fatal("report leaked another operator's transactions")
	}
	assertStatus(request("GET", "/api/v1/reports/reconciliations/export?type=invalid", "", "", owner), http.StatusUnprocessableEntity)
	assertStatus(request("POST", path, fmt.Sprintf(`{"type":"ASSIGN","assignee_id":%q}`, outsiderID), "application/json", assignee), http.StatusForbidden)
	if _, err := db.ExecContext(ctx, `UPDATE reconciliation_exceptions SET state='PENDING_ACTION_APPROVAL' WHERE id=$1`, exceptionID); err != nil {
		t.Fatal(err)
	}
	var manualApprovalID string
	queryID(&manualApprovalID, `INSERT INTO approval_requests(kind,batch_id,exception_id,run_id,source_version,maker_id,state,match_proposal_payload,reason) VALUES('MANUAL_MATCH',$1,$2,$3,1,$4,'PENDING',jsonb_build_object('internal_transaction_id',$5::text,'bank_transaction_id',$6::text),'Memeriksa perbedaan tanggal transaksi') RETURNING id::text`, batchID, exceptionID, runID, ownerID, internalID, bankID)
	assertStatus(request("POST", path, `{"type":"ASSIGN"}`, "application/json", owner), http.StatusConflict)
	assertStatus(request("POST", path, `{"type":"NOTE","note":"Evidence sedang diperiksa"}`, "application/json", assignee), http.StatusCreated)
	if err := db.QueryRowContext(ctx, `SELECT state FROM reconciliation_exceptions WHERE id=$1`, exceptionID).Scan(&state); err != nil || state != "PENDING_ACTION_APPROVAL" {
		t.Fatalf("pending state changed: %s, %v", state, err)
	}
	assertStatus(request("POST", "/api/v1/approvals/"+manualApprovalID+"/approve", "{}", "application/json", checker), http.StatusOK)
	if err := db.QueryRowContext(ctx, `SELECT state FROM reconciliation_exceptions WHERE id=$1`, exceptionID).Scan(&state); err != nil || state != "RESOLVED" {
		t.Fatalf("approved exception state: %s, %v", state, err)
	}
	var notifications int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND type='EXCEPTION_ASSIGNED' AND target_entity_id=$2`, assigneeID, exceptionID).Scan(&notifications); err != nil || notifications != 1 {
		t.Fatalf("assignment notifications=%d, error=%v", notifications, err)
	}
}

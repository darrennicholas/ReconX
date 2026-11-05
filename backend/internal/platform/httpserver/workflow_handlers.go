package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"reconx/backend/internal/modules/exceptions"
	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/modules/reconciliation"
	"reconx/backend/internal/platform/audit"
	"reconx/backend/internal/platform/httpx"
)

func (a *api) createBatch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AccountID  string `json:"account_id"`
		PeriodFrom string `json:"period_from"`
		PeriodTo   string `json:"period_to"`
		Remarks    string `json:"remarks"`
	}
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Data batch tidak sah.")
		return
	}
	from, fromErr := parseDate(input.PeriodFrom)
	to, toErr := parseDate(input.PeriodTo)
	if fromErr != nil || toErr != nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Tanggal periode harus berformat YYYY-MM-DD.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.reconciliation.CreateBatch(r.Context(), reconciliation.BatchInput{AccountID: input.AccountID, PeriodFrom: from, PeriodTo: to, Remarks: input.Remarks}, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.batchError(w, r, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", item.RowVersion))
	httpx.JSON(w, r, http.StatusCreated, item, nil)
}

func (a *api) listBatches(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	page, pageSize := pageArgs(r)
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	items, total, err := a.reconciliation.ListBatches(r.Context(), principal.UserID, isOperator(principal), r.URL.Query().Get("q"), r.URL.Query().Get("status"), r.URL.Query().Get("account_id"), from, to, page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) getBatch(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckBatchReadAccess(r.Context(), a.db, r.PathValue("id"), principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	item, err := a.reconciliation.GetBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", item.RowVersion))
	httpx.JSON(w, r, http.StatusOK, item, nil)
}

func (a *api) updateBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	var input struct {
		AccountID  string `json:"account_id"`
		PeriodFrom string `json:"period_from"`
		PeriodTo   string `json:"period_to"`
		Remarks    string `json:"remarks"`
	}
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Data batch tidak sah.")
		return
	}
	from, fromErr := parseDate(input.PeriodFrom)
	to, toErr := parseDate(input.PeriodTo)
	if fromErr != nil || toErr != nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Tanggal periode harus berformat YYYY-MM-DD.")
		return
	}
	item, err := a.reconciliation.UpdateDraft(r.Context(), r.PathValue("id"), version, reconciliation.BatchInput{AccountID: input.AccountID, PeriodFrom: from, PeriodTo: to, Remarks: input.Remarks}, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.batchError(w, r, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", item.RowVersion))
	httpx.JSON(w, r, http.StatusOK, item, nil)
}

func (a *api) cancelBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, true); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	if err := a.reconciliation.CancelDraft(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.batchError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"state": "CANCELLED"}, nil)
}

func (a *api) reopenBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, true); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	if err := a.reconciliation.ReopenForCorrection(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.batchError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"state": "DRAFT"}, nil)
}

func (a *api) retryBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := a.reconciliation.RetryFailedBatch(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.batchError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"state": "READY"}, nil)
}

func (a *api) uploadImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, reconciliation.MaxImportBytes+(1<<20))
	if err := r.ParseMultipartForm(reconciliation.MaxImportBytes + (1 << 20)); err != nil {
		httpx.Error(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran file maksimum 10 MB.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_required", "CSV wajib disertakan pada field file.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, reconciliation.MaxImportBytes+1))
	if err != nil || len(data) > reconciliation.MaxImportBytes {
		httpx.Error(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran file maksimum 10 MB.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	batchID := r.PathValue("id")
	if err := reconciliation.CheckOwnership(r.Context(), a.db, batchID, principal.UserID, true); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	item, err := a.reconciliation.UploadCSV(r.Context(), batchID, r.FormValue("source_type"), header.Filename, data, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.importError(w, r, err)
		return
	}
	status := http.StatusCreated
	if item.State == "INVALID" {
		status = http.StatusUnprocessableEntity
	}
	httpx.JSON(w, r, status, item, nil)
}

func (a *api) listImports(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	items, err := a.reconciliation.ListImports(r.Context(), r.PathValue("id"))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (a *api) downloadImportErrors(w http.ResponseWriter, r *http.Request) {
	a.downloadArtifact(w, r, true)
}

func (a *api) downloadImportFile(w http.ResponseWriter, r *http.Request) {
	a.downloadArtifact(w, r, false)
}

func (a *api) downloadArtifact(w http.ResponseWriter, r *http.Request, errorsOnly bool) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	batchID, err := a.reconciliation.GetImportBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFoundOrError(w, r, err, "Import tidak ditemukan.")
		return
	}
	if err := reconciliation.CheckOwnership(r.Context(), a.db, batchID, principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Import tidak ditemukan.")
		return
	}
	var artifact reconciliation.StoredArtifact
	if errorsOnly {
		artifact, err = a.reconciliation.ErrorReport(r.Context(), r.PathValue("id"))
	} else {
		artifact, err = a.reconciliation.ImportArtifact(r.Context(), r.PathValue("id"))
	}
	if err != nil {
		a.notFoundOrError(w, r, err, "File tidak ditemukan.")
		return
	}
	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": artifact.Filename})
	w.Header().Set("Content-Type", artifact.MimeType)
	w.Header().Set("Content-Disposition", contentDisposition)
	http.ServeFile(w, r, artifact.Path)
}

func (a *api) runMatching(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.reconciliation.StartMatching(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.batchError(w, r, err)
		return
	}
	jobContext, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Minute)
	go func() {
		defer cancel()
		if _, err := a.reconciliation.CompleteMatching(jobContext, item, principal.UserID, requestID(r), remoteIP(r)); err != nil {
			a.logger.Error("matching run failed", "run_id", item.ID, "batch_id", item.BatchID, "error", err)
		}
	}()
	httpx.JSON(w, r, http.StatusAccepted, item, nil)
}

func (a *api) latestRun(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	item, err := a.reconciliation.LatestRun(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFoundOrError(w, r, err, "Matching run belum tersedia.")
		return
	}
	httpx.JSON(w, r, http.StatusOK, item, nil)
}

func (a *api) listMatches(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := reconciliation.CheckOwnership(r.Context(), a.db, r.PathValue("id"), principal.UserID, isOperator(principal)); err != nil {
		a.notFoundOrError(w, r, err, "Batch tidak ditemukan.")
		return
	}
	page, pageSize := pageArgs(r)
	items, total, err := a.reconciliation.ListMatchesPage(r.Context(), r.PathValue("id"), 0, r.URL.Query().Get("q"), strings.ToUpper(r.URL.Query().Get("status")), page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) submitBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	approvalID, err := a.reconciliation.SubmitBatch(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.batchError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, map[string]string{"approval_id": approvalID, "state": "PENDING_APPROVAL"}, nil)
}

func (a *api) closeBatch(w http.ResponseWriter, r *http.Request) {
	version, ok := requireRowVersion(w, r)
	if !ok {
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := a.exceptions.CloseApprovedBatch(r.Context(), r.PathValue("id"), version, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.batchError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"state": "CLOSED"}, nil)
}

func (a *api) listTransactions(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	page, pageSize := pageArgs(r)
	items, total, err := a.reconciliation.ListTransactions(r.Context(), reconciliation.TransactionSearch{
		Query: r.URL.Query().Get("q"), BatchID: r.URL.Query().Get("batch_id"), Source: strings.ToUpper(r.URL.Query().Get("source")), Status: strings.ToUpper(r.URL.Query().Get("status")),
		OperatorID: principal.UserID, Operator: isOperator(principal), Page: page, PageSize: pageSize,
	})
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) getTransaction(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.reconciliation.GetTransaction(r.Context(), r.PathValue("id"), principal.UserID, isOperator(principal))
	if err != nil {
		a.notFoundOrError(w, r, err, "Transaksi tidak ditemukan.")
		return
	}
	httpx.JSON(w, r, http.StatusOK, item, nil)
}

func (a *api) listExceptions(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	page, pageSize := pageArgs(r)
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	items, total, err := a.exceptions.List(r.Context(), exceptions.ExceptionFilter{
		Query: r.URL.Query().Get("q"), Type: strings.ToUpper(r.URL.Query().Get("type")), State: strings.ToUpper(r.URL.Query().Get("status")),
		BatchID: r.URL.Query().Get("batch_id"), AccountID: r.URL.Query().Get("account_id"), AssigneeID: r.URL.Query().Get("assignee_id"), From: from, To: to, AgeBucket: strings.ToUpper(r.URL.Query().Get("age")),
		OperatorID: principal.UserID, Operator: isOperator(principal), Page: page, PageSize: pageSize,
	})
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) exceptionSummary(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	items, err := a.exceptions.Summary(r.Context(), exceptions.ExceptionFilter{
		Query: r.URL.Query().Get("q"), Type: strings.ToUpper(r.URL.Query().Get("type")), State: strings.ToUpper(r.URL.Query().Get("status")),
		BatchID: r.URL.Query().Get("batch_id"), AccountID: r.URL.Query().Get("account_id"), AssigneeID: r.URL.Query().Get("assignee_id"), From: from, To: to, AgeBucket: strings.ToUpper(r.URL.Query().Get("age")),
		OperatorID: principal.UserID, Operator: isOperator(principal),
	})
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, nil)
}

func (a *api) listExceptionOperators(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageArgs(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var total int
	base := ` FROM users u JOIN roles r ON r.id=u.role_id WHERE u.active AND r.code='OPERATOR' AND ($1='' OR u.full_name ILIKE '%'||$1||'%')`
	if err := a.db.QueryRowContext(r.Context(), `SELECT count(*)`+base, q).Scan(&total); err != nil {
		a.internalError(w, r, err)
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT u.id::text,u.full_name`+base+` ORDER BY u.full_name,u.id LIMIT $2 OFFSET $3`, q, pageSize, (page-1)*pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	defer rows.Close()
	type operator struct {
		ID       string `json:"id"`
		FullName string `json:"full_name"`
	}
	items := make([]operator, 0)
	for rows.Next() {
		var item operator
		if err := rows.Scan(&item.ID, &item.FullName); err != nil {
			a.internalError(w, r, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) getException(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.exceptions.Detail(r.Context(), r.PathValue("id"), principal.UserID, isOperator(principal))
	if err != nil {
		a.notFoundOrError(w, r, err, "Exception tidak ditemukan atau di luar cakupan akses.")
		return
	}
	httpx.JSON(w, r, http.StatusOK, item, nil)
}

func (a *api) addExceptionAction(w http.ResponseWriter, r *http.Request) {
	var input exceptions.ActionInput
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Aksi exception tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.exceptions.AddAction(r.Context(), r.PathValue("id"), input, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.exceptionError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, item, nil)
}

func (a *api) uploadExceptionEvidence(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (5<<20)+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "File evidence tidak valid atau melebihi batas 5 MB.")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_required", "Pilih file evidence terlebih dahulu.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 5<<20 {
		httpx.Error(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Evidence wajib berukuran maksimal 5 MB.")
		return
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	contentType := strings.Split(http.DetectContentType(sample), ";")[0]
	if contentType != "application/pdf" && contentType != "image/jpeg" && contentType != "image/png" && contentType != "text/plain" {
		httpx.Error(w, r, http.StatusUnsupportedMediaType, "unsupported_evidence_type", "Evidence hanya menerima PDF, JPEG, PNG, atau TXT.")
		return
	}
	filename := strings.TrimSpace(strings.ReplaceAll(header.Filename, "\\", "/"))
	parts := strings.Split(filename, "/")
	filename = parts[len(parts)-1]
	if filename == "" || len(filename) > 255 {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_filename", "Nama file evidence tidak valid.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.exceptions.AddEvidence(r.Context(), r.PathValue("id"), filename, contentType, data, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.exceptionError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, item, nil)
}

func (a *api) downloadExceptionEvidence(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	file, err := a.exceptions.DownloadEvidence(r.Context(), r.PathValue("id"), r.PathValue("evidenceID"), principal.UserID, isOperator(principal))
	if err != nil {
		a.notFoundOrError(w, r, err, "Evidence tidak ditemukan.")
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.FileName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, file.Path)
}

func (a *api) proposeManualMatch(w http.ResponseWriter, r *http.Request) {
	var input exceptions.ProposalInput
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Data proposal tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	item, err := a.exceptions.ProposeManualMatch(r.Context(), r.PathValue("id"), input.Reason, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.exceptionError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, item, nil)
}

func (a *api) listApprovals(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	if !identity.CanViewApprovals(principal.Role) {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "Akses approval hanya tersedia bagi Checker dan Auditor.")
		return
	}
	page, pageSize := pageArgs(r)
	items, total, err := a.exceptions.ListApprovalsPage(r.Context(), strings.ToUpper(r.URL.Query().Get("status")), strings.ToUpper(r.URL.Query().Get("kind")), r.URL.Query().Get("q"), page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) approveRequest(w http.ResponseWriter, r *http.Request) {
	a.decideApproval(w, r, "APPROVE")
}

func (a *api) rejectRequest(w http.ResponseWriter, r *http.Request) {
	a.decideApproval(w, r, "REJECT")
}

func (a *api) decideApproval(w http.ResponseWriter, r *http.Request, decision string) {
	var input struct {
		Comment string `json:"comment"`
	}
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Komentar keputusan tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	permission, err := a.exceptions.ApprovePermission(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFoundOrError(w, r, err, "Approval tidak ditemukan.")
		return
	}
	var makerID string
	if err := a.db.QueryRowContext(r.Context(), `SELECT maker_id::text FROM approval_requests WHERE id=$1`, r.PathValue("id")).Scan(&makerID); err != nil {
		a.notFoundOrError(w, r, err, "Approval tidak ditemukan.")
		return
	}
	if err := identity.AuthorizeApproval(principal, makerID, permission); err != nil {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "Checker tidak boleh memutuskan permintaannya sendiri.")
		return
	}
	if err := a.exceptions.DecideApproval(r.Context(), r.PathValue("id"), decision, input.Comment, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.exceptionError(w, r, err)
		return
	}
	state := "REJECTED"
	if decision == "APPROVE" {
		state = "APPROVED"
	}
	httpx.JSON(w, r, http.StatusOK, map[string]string{"state": state}, nil)
}

func (a *api) dashboard(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	var fromDate, toDate time.Time
	var fromErr, toErr error
	if from != "" {
		fromDate, fromErr = parseDate(from)
	}
	if to != "" {
		toDate, toErr = parseDate(to)
	}
	if fromErr != nil || toErr != nil || (from != "" && to != "" && fromDate.After(toDate)) {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Rentang tanggal dashboard tidak sah.")
		return
	}
	result, err := a.reconciliation.Dashboard(r.Context(), principal.UserID, isOperator(principal), from, to, r.URL.Query().Get("account_id"))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	result.OpenExceptions, result.ExceptionsOlder24h, err = a.reconciliation.OpenExceptionSummary(r.Context(), principal.UserID, isOperator(principal), from, to, r.URL.Query().Get("account_id"))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	result.PendingApprovals, err = a.reconciliation.PendingApprovals(r.Context(), principal.UserID, isOperator(principal), from, to, r.URL.Query().Get("account_id"))
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, result, nil)
}

func (a *api) report(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	page, pageSize := pageArgs(r)
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	q, status, accountID := r.URL.Query().Get("q"), r.URL.Query().Get("status"), r.URL.Query().Get("account_id")
	items, total, err := a.reconciliation.ListBatches(r.Context(), principal.UserID, isOperator(principal), q, status, accountID, from, to, page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	var summary struct {
		TotalBatches        int                             `json:"total_batches"`
		InternalCount       int                             `json:"internal_count"`
		BankCount           int                             `json:"bank_count"`
		MatchedPairs        int                             `json:"matched_pairs"`
		UnmatchedInternal   int                             `json:"unmatched_internal"`
		ActiveExceptions    int                             `json:"active_exceptions"`
		MatchRate           float64                         `json:"match_rate"`
		BatchStatuses       []reconciliation.DashboardCount `json:"batch_statuses"`
		ExceptionReasons    []reconciliation.DashboardCount `json:"exception_reasons"`
		AverageMatchSeconds float64                         `json:"average_match_seconds"`
	}
	statusCounts := map[string]int{}
	exceptionCounts := map[string]int{}
	var elapsedTotal float64
	var elapsedRuns int
	for current := 1; ; current++ {
		batches, count, listErr := a.reconciliation.ListBatches(r.Context(), principal.UserID, isOperator(principal), q, status, accountID, from, to, current, 100)
		if listErr != nil {
			a.internalError(w, r, listErr)
			return
		}
		for _, batch := range batches {
			summary.TotalBatches++
			summary.InternalCount += batch.InternalCount
			summary.BankCount += batch.BankCount
			summary.MatchedPairs += batch.MatchedPairs
			summary.UnmatchedInternal += max(0, batch.InternalCount-batch.MatchedPairs)
			summary.ActiveExceptions += batch.ActiveExceptions
			statusCounts[batch.State]++
			reasons, summaryErr := a.exceptions.Summary(r.Context(), exceptions.ExceptionFilter{BatchID: batch.ID, OperatorID: principal.UserID, Operator: isOperator(principal)})
			if summaryErr != nil {
				a.internalError(w, r, summaryErr)
				return
			}
			for _, reason := range reasons {
				exceptionCounts[reason.Name] += reason.Count
			}
			var average sql.NullFloat64
			var runCount int
			if err := a.db.QueryRowContext(r.Context(), `SELECT avg(EXTRACT(EPOCH FROM ended_at-started_at)),count(*) FROM reconciliation_runs WHERE batch_id=$1 AND state='SUCCEEDED' AND ended_at IS NOT NULL`, batch.ID).Scan(&average, &runCount); err != nil {
				a.internalError(w, r, err)
				return
			}
			if average.Valid && runCount > 0 {
				elapsedTotal += average.Float64 * float64(runCount)
				elapsedRuns += runCount
			}
		}
		if current*100 >= count {
			break
		}
	}
	if summary.InternalCount > 0 {
		summary.MatchRate = float64(summary.MatchedPairs) * 100 / float64(summary.InternalCount)
	}
	for name, count := range statusCounts {
		summary.BatchStatuses = append(summary.BatchStatuses, reconciliation.DashboardCount{Name: name, Count: count})
	}
	for name, count := range exceptionCounts {
		summary.ExceptionReasons = append(summary.ExceptionReasons, reconciliation.DashboardCount{Name: name, Count: count})
	}
	if elapsedRuns > 0 {
		summary.AverageMatchSeconds = elapsedTotal / float64(elapsedRuns)
	}
	httpx.JSON(w, r, http.StatusOK, map[string]any{"items": items, "summary": summary}, paginationMeta(page, pageSize, total))
}

func reportPeriod(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	from, to := strings.TrimSpace(r.URL.Query().Get("from")), strings.TrimSpace(r.URL.Query().Get("to"))
	for _, item := range []*string{&from, &to} {
		if *item == "" {
			continue
		}
		parsed, err := parseDate(*item)
		if err != nil {
			httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Parameter periode harus berformat YYYY-MM-DD.")
			return "", "", false
		}
		*item = parsed.Format("2006-01-02")
	}
	if from != "" && to != "" && from > to {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Periode from tidak boleh melewati to.")
		return "", "", false
	}
	return from, to, true
}

func (a *api) exportReport(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	reportType := r.URL.Query().Get("type")
	if reportType == "" {
		reportType = "all"
	}
	if reportType != "all" && reportType != "batches" && reportType != "transactions" && reportType != "exceptions" {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_report_type", "Jenis laporan tidak valid.")
		return
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	writeRow := func(row []string) {
		for len(row) < 21 {
			row = append(row, "")
		}
		for i, value := range row {
			trimmed := strings.TrimLeft(value, " \t\r\n")
			if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
				row[i] = "'" + value
			}
		}
		_ = writer.Write(row)
	}
	writeRow([]string{"record_type", "batch_no", "account", "period_from", "period_to", "source", "reference_id", "date", "direction", "amount_rupiah", "status", "exception_type", "actor", "action", "note", "timestamp", "internal_count", "bank_count", "matched_pairs", "active_exceptions", "match_rate_percent"})
	for page := 1; ; page++ {
		batches, totalBatches, err := a.reconciliation.ListBatches(r.Context(), principal.UserID, isOperator(principal), r.URL.Query().Get("q"), r.URL.Query().Get("status"), r.URL.Query().Get("account_id"), from, to, page, 100)
		if err != nil {
			a.internalError(w, r, err)
			return
		}
		for _, batch := range batches {
			if reportType == "all" || reportType == "batches" {
				rate := 0.0
				if batch.InternalCount > 0 {
					rate = float64(batch.MatchedPairs) * 100 / float64(batch.InternalCount)
				}
				writeRow([]string{"BATCH", batch.BatchNo, batch.AccountCode, batch.PeriodFrom.Format("2006-01-02"), batch.PeriodTo.Format("2006-01-02"), "", "", "", "", "", batch.State, "", "", "", "", "", strconv.Itoa(batch.InternalCount), strconv.Itoa(batch.BankCount), strconv.Itoa(batch.MatchedPairs), strconv.Itoa(batch.ActiveExceptions), strconv.FormatFloat(rate, 'f', 2, 64)})
			}
			if reportType == "all" || reportType == "transactions" {
				for txPage := 1; ; txPage++ {
					transactions, total, err := a.reconciliation.ListTransactions(r.Context(), reconciliation.TransactionSearch{BatchID: batch.ID, Page: txPage, PageSize: 100, OperatorID: principal.UserID, Operator: isOperator(principal)})
					if err != nil {
						a.internalError(w, r, err)
						return
					}
					for _, transaction := range transactions {
						writeRow([]string{"TRANSACTION", batch.BatchNo, batch.AccountCode, "", "", transaction.SourceType, transaction.ReferenceID, transaction.Date.Format("2006-01-02"), transaction.Direction, strconv.FormatInt(transaction.Amount, 10), transaction.Status, "", "", "", "", ""})
					}
					if txPage*100 >= total {
						break
					}
				}
			}
			if reportType == "all" || reportType == "exceptions" {
				for exceptionPage := 1; ; exceptionPage++ {
					exceptionsList, total, err := a.exceptions.List(r.Context(), exceptions.ExceptionFilter{BatchID: batch.ID, OperatorID: principal.UserID, Operator: isOperator(principal), Page: exceptionPage, PageSize: 100})
					if err != nil {
						a.internalError(w, r, err)
						return
					}
					for _, exception := range exceptionsList {
						writeRow([]string{"EXCEPTION", batch.BatchNo, batch.AccountCode, "", "", "", exception.ReferenceID, "", "", "", exception.State, exception.Type, "", "", "", ""})
						if reportType == "exceptions" {
							detail, err := a.exceptions.Detail(r.Context(), exception.ID, principal.UserID, isOperator(principal))
							if err != nil {
								a.internalError(w, r, err)
								return
							}
							for _, action := range detail.Actions {
								writeRow([]string{"EXCEPTION_ACTION", batch.BatchNo, batch.AccountCode, "", "", "", exception.ReferenceID, "", "", "", exception.State, exception.Type, action.ActorName, action.ActionType, action.Note, action.CreatedAt.Format(time.RFC3339)})
							}
							decisions, err := a.db.QueryContext(r.Context(), `SELECT a.state,COALESCE(u.full_name,''),a.reason,a.decision_comment,a.created_at,a.decided_at FROM approval_requests a LEFT JOIN users u ON u.id=a.checker_id WHERE a.exception_id=$1 ORDER BY a.created_at`, exception.ID)
							if err != nil {
								a.internalError(w, r, err)
								return
							}
							for decisions.Next() {
								var state, actor, reason, comment string
								var created time.Time
								var decided sql.NullTime
								if err := decisions.Scan(&state, &actor, &reason, &comment, &created, &decided); err != nil {
									decisions.Close()
									a.internalError(w, r, err)
									return
								}
								stamp := created
								if decided.Valid {
									stamp = decided.Time
								}
								writeRow([]string{"EXCEPTION_APPROVAL", batch.BatchNo, batch.AccountCode, "", "", "", exception.ReferenceID, "", "", "", state, exception.Type, actor, "MANUAL_MATCH", strings.TrimSpace(reason + " " + comment), stamp.Format(time.RFC3339)})
							}
							if err := decisions.Err(); err != nil {
								decisions.Close()
								a.internalError(w, r, err)
								return
							}
							decisions.Close()
						}
					}
					if exceptionPage*100 >= total {
						break
					}
				}
			}
		}
		if page*100 >= totalBatches {
			break
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		a.internalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="reconx-`+reportType+`.csv"`)
	_, _ = w.Write(output.Bytes())
}

func (a *api) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageArgs(r)
	q, entity, actor, from, to := r.URL.Query().Get("q"), r.URL.Query().Get("entity"), r.URL.Query().Get("actor"), r.URL.Query().Get("from"), r.URL.Query().Get("to")
	from, to, ok := reportPeriod(w, r)
	if !ok {
		return
	}
	where := ` WHERE ($1='' OR l.action ILIKE '%'||$1||'%' OR l.entity ILIKE '%'||$1||'%' OR l.entity_id ILIKE '%'||$1||'%')
		AND ($2='' OR l.entity=$2) AND ($3='' OR l.actor_id::text=$3 OR u.full_name ILIKE '%'||$3||'%')
		AND ($4='' OR l.created_at::date >= $4::date) AND ($5='' OR l.created_at::date <= $5::date)`
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT count(*) FROM audit_logs l LEFT JOIN users u ON u.id=l.actor_id`+where, q, entity, actor, from, to).Scan(&total); err != nil {
		a.internalError(w, r, err)
		return
	}
	query := `SELECT l.id::text,COALESCE(u.full_name,'System') actor,l.action,l.entity,l.entity_id,l.before_json,l.after_json,l.request_id,COALESCE(l.ip_address::text,''),l.created_at
		FROM audit_logs l LEFT JOIN users u ON u.id=l.actor_id` + where + ` ORDER BY l.created_at DESC LIMIT $6 OFFSET $7`
	rows, err := a.db.QueryContext(r.Context(), query, q, entity, actor, from, to, pageSize, (page-1)*pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	defer rows.Close()
	type logRow struct {
		ID        string          `json:"id"`
		Actor     string          `json:"actor"`
		Action    string          `json:"action"`
		Entity    string          `json:"entity"`
		EntityID  string          `json:"entity_id"`
		Before    json.RawMessage `json:"before,omitempty"`
		After     json.RawMessage `json:"after,omitempty"`
		RequestID string          `json:"request_id"`
		IP        string          `json:"ip,omitempty"`
		CreatedAt time.Time       `json:"created_at"`
	}
	items := make([]logRow, 0)
	for rows.Next() {
		var item logRow
		var before, after []byte
		if err := rows.Scan(&item.ID, &item.Actor, &item.Action, &item.Entity, &item.EntityID, &before, &after, &item.RequestID, &item.IP, &item.CreatedAt); err != nil {
			a.internalError(w, r, err)
			return
		}
		item.Before, item.After = before, after
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, items, paginationMeta(page, pageSize, total))
}

func (a *api) listNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	page, pageSize := pageArgs(r)
	rows, err := a.db.QueryContext(r.Context(), `SELECT id::text,type,title,body,target_entity,target_entity_id,read_at,created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, principal.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	defer rows.Close()
	type notification struct {
		ID             string     `json:"id"`
		Type           string     `json:"type"`
		Title          string     `json:"title"`
		Body           string     `json:"body"`
		TargetEntity   string     `json:"target_entity"`
		TargetEntityID string     `json:"target_entity_id"`
		ReadAt         *time.Time `json:"read_at,omitempty"`
		CreatedAt      time.Time  `json:"created_at"`
	}
	items := make([]notification, 0)
	for rows.Next() {
		var item notification
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Body, &item.TargetEntity, &item.TargetEntityID, &item.ReadAt, &item.CreatedAt); err != nil {
			a.internalError(w, r, err)
			return
		}
		items = append(items, item)
	}
	var unread int
	_ = a.db.QueryRowContext(r.Context(), `SELECT count(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, principal.UserID).Scan(&unread)
	httpx.JSON(w, r, http.StatusOK, items, map[string]int{"unread": unread, "page": page, "page_size": pageSize})
}

func (a *api) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	result, err := a.db.ExecContext(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND user_id=$2`, r.PathValue("id"), principal.UserID)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Notifikasi tidak ditemukan.")
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]bool{"read": true}, nil)
}

func (a *api) getSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `SELECT setting_key,setting_value,version,updated_at FROM settings ORDER BY setting_key`)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	defer rows.Close()
	type setting struct {
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
		Version   int64           `json:"version"`
		UpdatedAt time.Time       `json:"updated_at"`
	}
	items := make([]setting, 0)
	for rows.Next() {
		var item setting
		if err := rows.Scan(&item.Key, &item.Value, &item.Version, &item.UpdatedAt); err != nil {
			a.internalError(w, r, err)
			return
		}
		items = append(items, item)
	}
	httpx.JSON(w, r, http.StatusOK, items, map[string]bool{"matching_rules_read_only": true})
}

func (a *api) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Timezone       *string `json:"timezone"`
		MaxImportBytes *int64  `json:"max_import_bytes"`
		CSVDelimiter   *string `json:"csv_delimiter"`
	}
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Pengaturan tidak sah.")
		return
	}
	if input.Timezone == nil && input.MaxImportBytes == nil && input.CSVDelimiter == nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Tidak ada pengaturan yang dapat diubah.")
		return
	}
	if input.Timezone != nil && *input.Timezone != "Asia/Jakarta" {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "MVP v1 hanya mendukung timezone Asia/Jakarta.")
		return
	}
	if input.MaxImportBytes != nil && (*input.MaxImportBytes < 1024 || *input.MaxImportBytes > 10<<20) {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Ukuran import harus di antara 1 KB dan 10 MB.")
		return
	}
	if input.CSVDelimiter != nil && *input.CSVDelimiter != "," {
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "CSV delimiter v1 tetap koma.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	defer tx.Rollback()
	changes := make(map[string]any)
	apply := func(key string, value any) error {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO settings(setting_key,setting_value,version,updated_by,updated_at) VALUES ($1,$2::jsonb,1,$3,now()) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,version=settings.version+1,updated_by=EXCLUDED.updated_by,updated_at=now()`, key, string(jsonValue), principal.UserID); err != nil {
			return err
		}
		changes[key] = value
		return nil
	}
	if input.Timezone != nil {
		if err := apply("timezone", *input.Timezone); err != nil {
			a.internalError(w, r, err)
			return
		}
	}
	if input.MaxImportBytes != nil {
		if err := apply("max_import_bytes", *input.MaxImportBytes); err != nil {
			a.internalError(w, r, err)
			return
		}
	}
	if input.CSVDelimiter != nil {
		if err := apply("csv_delimiter", *input.CSVDelimiter); err != nil {
			a.internalError(w, r, err)
			return
		}
	}
	if err := audit.Append(r.Context(), tx, principal.UserID, "SETTINGS_UPDATED", "settings", "workspace", nil, changes, requestID(r), remoteIP(r)); err != nil {
		a.internalError(w, r, err)
		return
	}
	if err := tx.Commit(); err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, changes, nil)
}

func (a *api) batchError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, reconciliation.ErrInvalidPeriod):
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_period", "Periode batch tidak sah.")
	case errors.Is(err, reconciliation.ErrBatchOverlap):
		httpx.Error(w, r, http.StatusConflict, "period_overlap", "Periode bertindih dengan batch lain pada rekening yang sama.")
	case errors.Is(err, reconciliation.ErrBatchConflict), errors.Is(err, reconciliation.ErrRunNotReady), errors.Is(err, reconciliation.ErrSubmitBlocked):
		httpx.Error(w, r, http.StatusConflict, "state_conflict", "State batch berubah atau belum memenuhi prasyarat aksi.")
	case errors.Is(err, reconciliation.ErrForbiddenScope):
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "Batch berada di luar cakupan Operator.")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Batch tidak ditemukan.")
	default:
		a.internalError(w, r, err)
	}
}

func (a *api) importError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, reconciliation.ErrDuplicateImport):
		httpx.Error(w, r, http.StatusConflict, "duplicate_file", "File yang sama sudah diunggah untuk sumber dan batch ini.")
	case errors.Is(err, reconciliation.ErrCSVInvalid):
		httpx.Error(w, r, http.StatusUnprocessableEntity, "invalid_csv", "File harus CSV UTF-8 dan berukuran maksimum 10 MB.")
	case errors.Is(err, reconciliation.ErrBatchConflict):
		httpx.Error(w, r, http.StatusConflict, "state_conflict", "Import hanya dapat dilakukan pada batch DRAFT.")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Batch tidak ditemukan.")
	default:
		a.internalError(w, r, err)
	}
}

func (a *api) exceptionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, reconciliation.ErrForbiddenScope):
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "Exception berada di luar cakupan Operator.")
	case errors.Is(err, exceptions.ErrInvalidAction), errors.Is(err, exceptions.ErrNotEligible):
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	case errors.Is(err, exceptions.ErrApprovalConflict), errors.Is(err, reconciliation.ErrBatchConflict):
		httpx.Error(w, r, http.StatusConflict, "state_conflict", "Approval atau snapshot sudah berubah.")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Exception atau approval tidak ditemukan.")
	default:
		a.internalError(w, r, err)
	}
}

func pageArgs(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	return page, pageSize
}

func paginationMeta(page, pageSize, total int) map[string]int {
	return map[string]int{"page": page, "page_size": pageSize, "total": total}
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func requireRowVersion(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), "\"")
	if value == "" {
		httpx.Error(w, r, http.StatusPreconditionRequired, "if_match_required", "Sertakan row_version terbaru pada header If-Match.")
		return 0, false
	}
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 1 {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_if_match", "Header If-Match tidak sah.")
		return 0, false
	}
	return version, true
}

func isOperator(principal identity.Principal) bool { return principal.Role == identity.RoleOperator }

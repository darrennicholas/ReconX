package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"reconx/backend/internal/modules/banking"
	"reconx/backend/internal/modules/exceptions"
	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/modules/reconciliation"
	"reconx/backend/internal/platform/httpx"
)

type api struct {
	db             *sql.DB
	logger         *slog.Logger
	identity       identity.Service
	banking        banking.Service
	reconciliation reconciliation.Service
	exceptions     exceptions.Service
	loginLimiter   *loginLimiter
	secureCookie   bool
}

type sessionState struct {
	id       string
	csrfHash []byte
}

type sessionContextKey struct{}

func registerFeatureRoutes(mux *http.ServeMux, db *sql.DB, logger *slog.Logger) {
	app := &api{
		db: db, logger: logger, identity: identity.Service{DB: db}, banking: banking.Service{DB: db},
		reconciliation: reconciliation.Service{DB: db, StoragePath: os.Getenv("IMPORT_STORAGE_DIR")}, exceptions: exceptions.Service{DB: db, StoragePath: os.Getenv("IMPORT_STORAGE_DIR")},
		loginLimiter: newLoginLimiter(), secureCookie: strings.EqualFold(os.Getenv("APP_ENV"), "production"),
	}

	mux.HandleFunc("POST /api/v1/auth/login", app.login)
	mux.Handle("POST /api/v1/auth/logout", app.protect("", true, app.logout))
	mux.Handle("GET /api/v1/auth/me", app.protect("", false, app.me))

	mux.Handle("GET /api/v1/users", app.protect(identity.PermissionUsersRead, false, app.listUsers))
	mux.Handle("POST /api/v1/users", app.protectIdempotent(identity.PermissionUsersManage, app.createUser))
	mux.Handle("GET /api/v1/users/{id}", app.protect(identity.PermissionUsersRead, false, app.getUser))
	mux.Handle("PATCH /api/v1/users/{id}", app.protect(identity.PermissionUsersManage, true, app.updateUser))
	mux.Handle("POST /api/v1/users/{id}/reset-password", app.protect(identity.PermissionUsersManage, true, app.resetPassword))

	mux.Handle("GET /api/v1/bank-accounts", app.protect(identity.PermissionAccountsRead, false, app.listAccounts))
	mux.Handle("POST /api/v1/bank-accounts", app.protectIdempotent(identity.PermissionAccountsManage, app.createAccount))
	mux.Handle("GET /api/v1/bank-accounts/{id}", app.protect(identity.PermissionAccountsRead, false, app.getAccount))
	mux.Handle("PATCH /api/v1/bank-accounts/{id}", app.protect(identity.PermissionAccountsManage, true, app.updateAccount))

	mux.Handle("GET /api/v1/batches", app.protect(identity.PermissionBatchesRead, false, app.listBatches))
	mux.Handle("POST /api/v1/batches", app.protectIdempotent(identity.PermissionBatchesCreate, app.createBatch))
	mux.Handle("GET /api/v1/batches/{id}", app.protect(identity.PermissionBatchesRead, false, app.getBatch))
	mux.Handle("PATCH /api/v1/batches/{id}", app.protect(identity.PermissionBatchesEdit, true, app.updateBatch))
	mux.Handle("POST /api/v1/batches/{id}/cancel", app.protectIdempotent(identity.PermissionBatchesEdit, app.cancelBatch))
	mux.Handle("POST /api/v1/batches/{id}/reopen", app.protectIdempotent(identity.PermissionBatchesEdit, app.reopenBatch))
	mux.Handle("POST /api/v1/batches/{id}/retry", app.protectIdempotent(identity.PermissionMatchingRun, app.retryBatch))
	mux.Handle("POST /api/v1/batches/{id}/imports", app.protectIdempotent(identity.PermissionImportsManage, app.uploadImport))
	mux.Handle("GET /api/v1/batches/{id}/imports", app.protect(identity.PermissionBatchesRead, false, app.listImports))
	mux.Handle("GET /api/v1/imports/{id}/errors", app.protect(identity.PermissionBatchesRead, false, app.downloadImportErrors))
	mux.Handle("GET /api/v1/imports/{id}/file", app.protect(identity.PermissionBatchesRead, false, app.downloadImportFile))
	mux.Handle("POST /api/v1/batches/{id}/match-runs", app.protectIdempotent(identity.PermissionMatchingRun, app.runMatching))
	mux.Handle("GET /api/v1/batches/{id}/match-runs/latest", app.protect(identity.PermissionBatchesRead, false, app.latestRun))
	mux.Handle("GET /api/v1/batches/{id}/matches", app.protect(identity.PermissionBatchesRead, false, app.listMatches))
	mux.Handle("POST /api/v1/batches/{id}/submit", app.protectIdempotent(identity.PermissionBatchesSubmit, app.submitBatch))
	mux.Handle("POST /api/v1/batches/{id}/close", app.protectIdempotent(identity.PermissionBatchesClose, app.closeBatch))

	mux.Handle("GET /api/v1/transactions", app.protect(identity.PermissionBatchesRead, false, app.listTransactions))
	mux.Handle("GET /api/v1/transactions/{id}", app.protect(identity.PermissionBatchesRead, false, app.getTransaction))
	mux.Handle("GET /api/v1/exceptions", app.protect(identity.PermissionBatchesRead, false, app.listExceptions))
	mux.Handle("GET /api/v1/exceptions/summary", app.protect(identity.PermissionBatchesRead, false, app.exceptionSummary))
	mux.Handle("GET /api/v1/exception-operators", app.protect(identity.PermissionExceptionsInvestigate, false, app.listExceptionOperators))
	mux.Handle("GET /api/v1/exceptions/{id}", app.protect(identity.PermissionBatchesRead, false, app.getException))
	mux.Handle("POST /api/v1/exceptions/{id}/actions", app.protect(identity.PermissionExceptionsInvestigate, true, app.addExceptionAction))
	mux.Handle("POST /api/v1/exceptions/{id}/evidence", app.protectIdempotent(identity.PermissionExceptionsInvestigate, app.uploadExceptionEvidence))
	mux.Handle("GET /api/v1/exceptions/{id}/evidence/{evidenceID}", app.protect(identity.PermissionBatchesRead, false, app.downloadExceptionEvidence))
	mux.Handle("POST /api/v1/exceptions/{id}/manual-match-proposals", app.protectIdempotent(identity.PermissionMatchesPropose, app.proposeManualMatch))
	mux.Handle("GET /api/v1/approvals", app.protect("", false, app.listApprovals))
	mux.Handle("GET /api/v1/approvals/{id}", app.protect("", false, app.getApproval))
	mux.Handle("POST /api/v1/approvals/{id}/approve", app.protectIdempotent("", app.approveRequest))
	mux.Handle("POST /api/v1/approvals/{id}/reject", app.protectIdempotent("", app.rejectRequest))

	mux.Handle("GET /api/v1/dashboard/summary", app.protect(identity.PermissionBatchesRead, false, app.dashboard))
	mux.Handle("GET /api/v1/reports/reconciliations/export", app.protect(identity.PermissionReportsExport, false, app.exportReport))
	mux.Handle("GET /api/v1/reports/reconciliations", app.protect(identity.PermissionReportsRead, false, app.report))
	mux.Handle("GET /api/v1/audit-logs", app.protect(identity.PermissionAuditRead, false, app.listAuditLogs))
	mux.Handle("GET /api/v1/notifications", app.protect("", false, app.listNotifications))
	mux.Handle("PATCH /api/v1/notifications/{id}/read", app.protect("", true, app.markNotificationRead))
	mux.Handle("GET /api/v1/settings", app.protectSettings(false, app.getSettings))
	mux.Handle("PATCH /api/v1/settings", app.protectSettings(true, app.updateSettings))
}

func (a *api) protect(permission identity.Permission, csrf bool, handler http.HandlerFunc) http.Handler {
	var next http.Handler = handler
	if permission != "" {
		next = identity.RequirePermission(permission, next)
	}
	if csrf {
		next = a.requireCSRF(next)
	}
	return a.requireSession(next)
}

func (a *api) protectSettings(write bool, handler http.HandlerFunc) http.Handler {
	var next http.Handler = handler
	if write {
		next = identity.RequirePermission(identity.PermissionSettingsManage, next)
	} else {
		guarded := next
		next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, _ := identity.PrincipalFromContext(r.Context())
			if !identity.CanViewSettings(principal.Role) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "Akses tidak diizinkan.")
				return
			}
			guarded.ServeHTTP(w, r)
		})
	}
	if write {
		next = a.requireCSRF(next)
	}
	return a.requireSession(next)
}

func (a *api) protectIdempotent(permission identity.Permission, handler http.HandlerFunc) http.Handler {
	var next http.Handler = handler
	next = a.idempotent(next)
	if permission != "" {
		next = identity.RequirePermission(permission, next)
	}
	next = a.requireCSRF(next)
	return a.requireSession(next)
}

func (a *api) idempotent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		principal, _ := identity.PrincipalFromContext(r.Context())
		if len(key) < 8 || len(key) > 128 {
			httpx.Error(w, r, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key wajib berisi 8–128 karakter.")
			return
		}
		requestLimit := int64(reconciliation.MaxImportBytes + (1 << 20))
		body, err := io.ReadAll(io.LimitReader(r.Body, requestLimit+1))
		if err != nil || int64(len(body)) > requestLimit {
			httpx.Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "Ukuran request melebihi batas.")
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		requestHash := canonicalRequestHash(r, body)
		conn, err := a.db.Conn(r.Context())
		if err != nil {
			a.internalError(w, r, err)
			return
		}
		defer conn.Close()
		if _, err := conn.ExecContext(r.Context(), `SELECT pg_advisory_lock(hashtext($1),hashtext($2))`, principal.UserID, key); err != nil {
			a.internalError(w, r, err)
			return
		}
		defer func() {
			_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1),hashtext($2))`, principal.UserID, key)
		}()
		var path string
		var cachedStatus int
		var cachedBody []byte
		var cachedHash []byte
		err = conn.QueryRowContext(r.Context(), `SELECT request_path,request_hash,response_status,response_body FROM idempotency_keys WHERE user_id=$1 AND idempotency_key=$2`, principal.UserID, key).Scan(&path, &cachedHash, &cachedStatus, &cachedBody)
		if err == nil {
			if path != r.URL.Path || !equalBytes(cachedHash, requestHash) {
				httpx.Error(w, r, http.StatusConflict, "idempotency_key_reused", "Idempotency-Key sudah dipakai untuk request berbeda.")
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(cachedStatus)
			_, _ = w.Write(cachedBody)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			a.internalError(w, r, err)
			return
		}
		capture := newCaptureWriter()
		next.ServeHTTP(capture, r)
		for name, values := range capture.header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		status := capture.status
		if status == 0 {
			status = http.StatusOK
		}
		capturedBody := []byte(capture.body.String())
		w.WriteHeader(status)
		_, _ = w.Write(capturedBody)
		if status >= 200 && status < 500 && json.Valid(capturedBody) {
			_, err := conn.ExecContext(r.Context(), `INSERT INTO idempotency_keys(user_id,idempotency_key,request_path,request_hash,response_status,response_body) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, principal.UserID, key, r.URL.Path, requestHash, status, capture.body.String())
			if err != nil {
				a.logger.Error("store idempotency result", "request_id", requestID(r), "error", err)
			}
		}
	})
}

type captureWriter struct {
	header http.Header
	status int
	body   strings.Builder
}

func newCaptureWriter() *captureWriter       { return &captureWriter{header: make(http.Header)} }
func (w *captureWriter) Header() http.Header { return w.header }
func (w *captureWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *captureWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(value)
}

func equalBytes(left, right []byte) bool { return subtle.ConstantTimeCompare(left, right) == 1 }

type multipartDigestPart struct {
	name     string
	filename string
	data     []byte
}

func canonicalRequestHash(r *http.Request, body []byte) []byte {
	hasher := sha256.New()
	mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	writeDigestPart(hasher, []byte(r.Method+"\n"+r.URL.Path+"\n"+r.URL.RawQuery+"\n"+mediaType))
	if mediaType != "multipart/form-data" || params["boundary"] == "" {
		writeDigestPart(hasher, body)
		return hasher.Sum(nil)
	}
	reader := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	parts := make([]multipartDigestPart, 0, 2)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeDigestPart(hasher, body)
			return hasher.Sum(nil)
		}
		data, err := io.ReadAll(io.LimitReader(part, int64(reconciliation.MaxImportBytes+(1<<20))+1))
		_ = part.Close()
		if err != nil {
			writeDigestPart(hasher, body)
			return hasher.Sum(nil)
		}
		parts = append(parts, multipartDigestPart{name: part.FormName(), filename: part.FileName(), data: data})
	}
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].name != parts[j].name {
			return parts[i].name < parts[j].name
		}
		if parts[i].filename != parts[j].filename {
			return parts[i].filename < parts[j].filename
		}
		return string(parts[i].data) < string(parts[j].data)
	})
	for _, part := range parts {
		writeDigestPart(hasher, []byte(part.name))
		writeDigestPart(hasher, []byte(part.filename))
		writeDigestPart(hasher, part.data)
	}
	return hasher.Sum(nil)
}

func writeDigestPart(hasher hash.Hash, data []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(data)))
	_, _ = hasher.Write(size[:])
	_, _ = hasher.Write(data)
}

func (a *api) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(identity.SessionCookieName)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "Login diperlukan.")
			return
		}
		principal, sessionID, csrfHash, err := a.identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "Sesi tidak sah atau sudah tamat.")
			return
		}
		ctx := identity.WithPrincipal(r.Context(), principal)
		ctx = contextWithSession(ctx, sessionState{id: sessionID, csrfHash: csrfHash})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *api) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state, ok := sessionFromContext(r.Context())
		cookie, cookieErr := r.Cookie(identity.CSRFCookieName)
		header := r.Header.Get("X-CSRF-Token")
		if !ok || cookieErr != nil || header == "" || cookie.Value != header {
			httpx.Error(w, r, http.StatusForbidden, "csrf_invalid", "Token CSRF tidak sah.")
			return
		}
		provided := sha256.Sum256([]byte(header))
		if subtle.ConstantTimeCompare(provided[:], state.csrfHash) != 1 {
			httpx.Error(w, r, http.StatusForbidden, "csrf_invalid", "Token CSRF tidak sah.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	ip := remoteIP(r)
	if !a.loginLimiter.allow(ip) {
		httpx.Error(w, r, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan login. Coba lagi nanti.")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil || strings.TrimSpace(input.Email) == "" || input.Password == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Email dan password wajib diisi.")
		return
	}
	requestID := requestID(r)
	credentials, err := a.identity.Login(r.Context(), input.Email, input.Password, requestID, ip)
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			httpx.Error(w, r, http.StatusUnauthorized, "invalid_credentials", "Email atau password tidak benar.")
			return
		}
		a.internalError(w, r, err)
		return
	}
	a.loginLimiter.reset(ip)
	http.SetCookie(w, &http.Cookie{Name: identity.SessionCookieName, Value: credentials.SessionToken, Path: "/", Expires: credentials.ExpiresAt, MaxAge: int(identity.SessionDuration.Seconds()), HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: identity.CSRFCookieName, Value: credentials.CSRFToken, Path: "/", Expires: credentials.ExpiresAt, MaxAge: int(identity.SessionDuration.Seconds()), Secure: a.secureCookie, SameSite: http.SameSiteStrictMode})
	httpx.JSON(w, r, http.StatusOK, credentials.User, nil)
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	state, _ := sessionFromContext(r.Context())
	if err := a.identity.Logout(r.Context(), state.id, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.internalError(w, r, err)
		return
	}
	clearCookie(w, identity.SessionCookieName, true, a.secureCookie)
	clearCookie(w, identity.CSRFCookieName, false, a.secureCookie)
	httpx.JSON(w, r, http.StatusOK, map[string]bool{"logged_out": true}, nil)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := identity.PrincipalFromContext(r.Context())
	user, err := a.identity.Me(r.Context(), principal.UserID)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, user, nil)
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageArgs(r)
	users, total, err := a.identity.ListUsersPage(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("role"), r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, users, paginationMeta(page, pageSize, total))
}

func (a *api) getUser(w http.ResponseWriter, r *http.Request) {
	users, err := a.identity.ListUsers(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	for _, user := range users {
		if user.ID == r.PathValue("id") {
			httpx.JSON(w, r, http.StatusOK, user, nil)
			return
		}
	}
	httpx.Error(w, r, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FullName string        `json:"full_name"`
		Email    string        `json:"email"`
		Password string        `json:"password"`
		Role     identity.Role `json:"role"`
	}
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Request pengguna tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	user, err := a.identity.CreateUser(r.Context(), input.FullName, input.Email, input.Password, input.Role, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.identityError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, user, nil)
}

func (a *api) updateUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FullName string        `json:"full_name"`
		Email    string        `json:"email"`
		Role     identity.Role `json:"role"`
		Active   *bool         `json:"active"`
	}
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil || input.Active == nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Field active dan detail pengguna wajib diisi.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	user, err := a.identity.UpdateUser(r.Context(), r.PathValue("id"), input.FullName, input.Email, input.Role, *input.Active, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.identityError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, user, nil)
}

func (a *api) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, 16<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Password baru tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	if err := a.identity.ResetPassword(r.Context(), r.PathValue("id"), input.Password, principal.UserID, requestID(r), remoteIP(r)); err != nil {
		a.identityError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, map[string]bool{"password_reset": true}, nil)
}

func (a *api) listAccounts(w http.ResponseWriter, r *http.Request) {
	var active *bool
	if value := r.URL.Query().Get("active"); value != "" {
		parsed := value == "true"
		active = &parsed
	}
	page, pageSize := pageArgs(r)
	accounts, total, err := a.banking.ListAccountsPage(r.Context(), r.URL.Query().Get("q"), active, page, pageSize)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, accounts, paginationMeta(page, pageSize, total))
}

func (a *api) getAccount(w http.ResponseWriter, r *http.Request) {
	account, err := a.banking.GetAccount(r.Context(), r.PathValue("id"))
	if err != nil {
		a.notFoundOrError(w, r, err, "Rekening tidak ditemukan.")
		return
	}
	httpx.JSON(w, r, http.StatusOK, account, nil)
}

func (a *api) createAccount(w http.ResponseWriter, r *http.Request) {
	var input banking.AccountView
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Data rekening tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	account, err := a.banking.CreateAccount(r.Context(), input, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.accountError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, account, nil)
}

func (a *api) updateAccount(w http.ResponseWriter, r *http.Request) {
	var input banking.AccountView
	if err := httpx.DecodeJSON(w, r, 32<<10, &input); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Data rekening tidak sah.")
		return
	}
	principal, _ := identity.PrincipalFromContext(r.Context())
	account, err := a.banking.UpdateAccount(r.Context(), r.PathValue("id"), input, principal.UserID, requestID(r), remoteIP(r))
	if err != nil {
		a.accountError(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, account, nil)
}

func (a *api) internalError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.Error("API request failed", "request_id", requestID(r), "path", r.URL.Path, "error", err)
	httpx.Error(w, r, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server.")
}

func (a *api) identityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, identity.ErrWeakPassword), errors.Is(err, identity.ErrInvalidRole):
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	case errors.Is(err, identity.ErrEmailInUse):
		httpx.Error(w, r, http.StatusConflict, "email_in_use", "Email sudah digunakan.")
	case errors.Is(err, identity.ErrLastActiveAdmin):
		httpx.Error(w, r, http.StatusConflict, "last_active_admin", "Sistem harus memiliki minimal satu Admin aktif.")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Data tidak ditemukan.")
	default:
		a.internalError(w, r, err)
	}
}

func (a *api) accountError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, banking.ErrInvalidAccount):
		httpx.Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Lengkapi kode, nama bank, nomor tersamarkan, dan pemilik rekening.")
	case errors.Is(err, banking.ErrAccountInUse):
		httpx.Error(w, r, http.StatusConflict, "account_in_use", "Rekening masih mempunyai batch aktif.")
	case errors.Is(err, banking.ErrAccountExists):
		httpx.Error(w, r, http.StatusConflict, "account_exists", "Kode rekening sudah digunakan.")
	case errors.Is(err, sql.ErrNoRows):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Rekening tidak ditemukan.")
	default:
		a.internalError(w, r, err)
	}
}

func (a *api) notFoundOrError(w http.ResponseWriter, r *http.Request, err error, message string) {
	if errors.Is(err, reconciliation.ErrForbiddenScope) {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "Data berada di luar cakupan akses Anda.")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", message)
		return
	}
	a.internalError(w, r, err)
}

func clearCookie(w http.ResponseWriter, name string, httpOnly, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: httpOnly, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func requestID(r *http.Request) string {
	return r.Header.Get("X-Request-ID")
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func contextWithSession(ctx context.Context, state sessionState) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, state)
}

func sessionFromContext(ctx context.Context) (sessionState, bool) {
	state, ok := ctx.Value(sessionContextKey{}).(sessionState)
	return state, ok
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{attempts: make(map[string][]time.Time)} }

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-15 * time.Minute)
	kept := l.attempts[key][:0]
	for _, timestamp := range l.attempts[key] {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	if len(kept) >= 5 {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, time.Now())
	return true
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 100 {
			bytes := make([]byte, 16)
			if _, err := rand.Read(bytes); err == nil {
				id = hex.EncodeToString(bytes)
			} else {
				id = time.Now().UTC().Format("20060102150405.000000000")
			}
		}
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

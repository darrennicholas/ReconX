package identity

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPermissionMatrixEnforcedByMiddleware(t *testing.T) {
	cases := []struct {
		role       Role
		permission Permission
		allowed    bool
	}{
		{RoleAdmin, PermissionUsersManage, true}, {RoleOperator, PermissionUsersRead, false},
		{RoleAdmin, PermissionMatchingRun, false}, {RoleOperator, PermissionMatchingRun, true},
		{RoleChecker, PermissionBatchesApprove, true}, {RoleAdmin, PermissionBatchesApprove, false},
		{RoleAuditor, PermissionBatchesRead, true}, {RoleAuditor, PermissionBatchesCreate, false},
		{RoleAuditor, PermissionExceptionsInvestigate, false}, {RoleAuditor, PermissionBatchesClose, false},
		{RoleChecker, PermissionReportsExport, true}, {RoleOperator, PermissionAuditRead, false},
	}
	for _, tt := range cases {
		t.Run(string(tt.role)+"/"+string(tt.permission), func(t *testing.T) {
			called := false
			handler := RequirePermission(tt.permission, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(WithPrincipal(req.Context(), Principal{UserID: "user", Role: tt.role, IsActive: true}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			want := http.StatusForbidden
			if tt.allowed {
				want = http.StatusNoContent
			}
			if rec.Code != want || called != tt.allowed {
				t.Fatalf("status=%d called=%v; want status=%d called=%v", rec.Code, called, want, tt.allowed)
			}
		})
	}
}

func TestApprovalRequiresIndependentChecker(t *testing.T) {
	for _, permission := range []Permission{PermissionMatchesApprove, PermissionBatchesApprove, PermissionBatchesClose} {
		if err := AuthorizeApproval(Principal{UserID: "maker", Role: RoleChecker, IsActive: true}, "maker", permission); !errors.Is(err, ErrSelfApproval) {
			t.Errorf("%s self approval: %v", permission, err)
		}
		if err := AuthorizeApproval(Principal{UserID: "admin", Role: RoleAdmin, IsActive: true}, "maker", permission); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s admin bypass: %v", permission, err)
		}
		if err := AuthorizeApproval(Principal{UserID: "checker", Role: RoleChecker, IsActive: true}, "maker", permission); err != nil {
			t.Errorf("%s independent checker: %v", permission, err)
		}
	}
}

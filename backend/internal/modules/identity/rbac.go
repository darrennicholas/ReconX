package identity

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"reconx/backend/internal/platform/httpx"
)

type Role string
type Permission string

const (
	RoleAdmin    Role = "ADMIN"
	RoleOperator Role = "OPERATOR"
	RoleChecker  Role = "CHECKER"
	RoleAuditor  Role = "AUDITOR"
)

const (
	PermissionUsersRead             Permission = "users.read"
	PermissionUsersManage           Permission = "users.manage"
	PermissionAccountsRead          Permission = "accounts.read"
	PermissionAccountsManage        Permission = "accounts.manage"
	PermissionBatchesRead           Permission = "batches.read"
	PermissionBatchesCreate         Permission = "batches.create"
	PermissionBatchesEdit           Permission = "batches.edit"
	PermissionImportsManage         Permission = "imports.manage"
	PermissionMatchingRun           Permission = "matching.run"
	PermissionExceptionsInvestigate Permission = "exceptions.investigate"
	PermissionMatchesPropose        Permission = "matches.propose"
	PermissionMatchesApprove        Permission = "matches.approve"
	PermissionBatchesSubmit         Permission = "batches.submit"
	PermissionBatchesApprove        Permission = "batches.approve"
	PermissionBatchesClose          Permission = "batches.close"
	PermissionReportsRead           Permission = "reports.read"
	PermissionReportsExport         Permission = "reports.export"
	PermissionAuditRead             Permission = "audit.read"
	PermissionSettingsManage        Permission = "settings.manage"
)

var permissionKeys = []Permission{
	PermissionUsersRead,
	PermissionUsersManage,
	PermissionAccountsRead,
	PermissionAccountsManage,
	PermissionBatchesRead,
	PermissionBatchesCreate,
	PermissionBatchesEdit,
	PermissionImportsManage,
	PermissionMatchingRun,
	PermissionExceptionsInvestigate,
	PermissionMatchesPropose,
	PermissionMatchesApprove,
	PermissionBatchesSubmit,
	PermissionBatchesApprove,
	PermissionBatchesClose,
	PermissionReportsRead,
	PermissionReportsExport,
	PermissionAuditRead,
	PermissionSettingsManage,
}

var rolePermissions = map[Role]map[Permission]struct{}{
	RoleAdmin: permissionSet(
		PermissionUsersRead, PermissionUsersManage,
		PermissionAccountsRead, PermissionAccountsManage,
		PermissionBatchesRead, PermissionReportsRead, PermissionReportsExport,
		PermissionAuditRead, PermissionSettingsManage,
	),
	RoleOperator: permissionSet(
		PermissionAccountsRead, PermissionBatchesRead, PermissionBatchesCreate,
		PermissionBatchesEdit, PermissionImportsManage, PermissionMatchingRun,
		PermissionExceptionsInvestigate, PermissionMatchesPropose, PermissionBatchesSubmit,
		PermissionReportsRead, PermissionReportsExport,
	),
	RoleChecker: permissionSet(
		PermissionAccountsRead, PermissionBatchesRead, PermissionMatchesApprove,
		PermissionBatchesApprove, PermissionBatchesClose, PermissionReportsRead,
		PermissionReportsExport, PermissionAuditRead,
	),
	RoleAuditor: permissionSet(
		PermissionAccountsRead, PermissionBatchesRead, PermissionReportsRead,
		PermissionReportsExport, PermissionAuditRead,
	),
}

var ErrUnauthenticated = errors.New("unauthenticated or inactive principal")
var ErrForbidden = errors.New("permission denied")
var ErrSelfApproval = errors.New("maker cannot approve their own request")

type Principal struct {
	UserID   string
	Role     Role
	IsActive bool
}

type RoleDefinition struct {
	Key              Role         `json:"key"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Permissions      []Permission `json:"permissions"`
	CanViewDashboard bool         `json:"can_view_dashboard"`
	CanViewResults   bool         `json:"can_view_results"`
	CanViewApprovals bool         `json:"can_view_approvals"`
	CanViewSettings  bool         `json:"can_view_settings"`
}

type contextKey struct{}

func permissionSet(permissions ...Permission) map[Permission]struct{} {
	set := make(map[Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		set[permission] = struct{}{}
	}
	return set
}

// PermissionKeys returns the fixed permission registry defined by FSD section 3.3.
func PermissionKeys() []Permission {
	return append([]Permission(nil), permissionKeys...)
}

// RoleDefinitions returns the fixed MVP roles and their permission keys.
func RoleDefinitions() []RoleDefinition {
	names := []struct {
		role        Role
		name        string
		description string
	}{
		{RoleAdmin, "Admin", "Mengelola pengguna, rekening referensi, dan konfigurasi sistem."},
		{RoleOperator, "Operator", "Membuat batch, mengimpor data, menjalankan matching, dan menyelidiki exception."},
		{RoleChecker, "Checker", "Meninjau hasil dan menyetujui tindakan secara independen dari maker."},
		{RoleAuditor, "Auditor", "Akses read-only ke batch, transaksi, approval, laporan, dan audit log."},
	}

	definitions := make([]RoleDefinition, 0, len(names))
	for _, item := range names {
		definitions = append(definitions, RoleDefinition{
			Key:              item.role,
			Name:             item.name,
			Description:      item.description,
			Permissions:      PermissionsFor(item.role),
			CanViewDashboard: CanViewDashboard(item.role),
			CanViewResults:   CanViewResults(item.role),
			CanViewApprovals: CanViewApprovals(item.role),
			CanViewSettings:  CanViewSettings(item.role),
		})
	}
	return definitions
}

func IsRole(role Role) bool {
	_, ok := rolePermissions[role]
	return ok
}

func HasPermission(role Role, permission Permission) bool {
	_, ok := rolePermissions[role][permission]
	return ok
}

func PermissionsFor(role Role) []Permission {
	set := rolePermissions[role]
	permissions := make([]Permission, 0, len(set))
	for permission := range set {
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool { return permissions[i] < permissions[j] })
	return permissions
}

func CanViewDashboard(role Role) bool { return IsRole(role) }

func CanViewResults(role Role) bool { return IsRole(role) }

func CanViewApprovals(role Role) bool { return role == RoleChecker || role == RoleAuditor }

func CanViewSettings(role Role) bool {
	return role == RoleAdmin || role == RoleChecker || role == RoleAuditor
}

// Authorize checks authentication state, role validity, and the requested permission.
func Authorize(principal Principal, permission Permission) error {
	if principal.UserID == "" || !principal.IsActive || !IsRole(principal.Role) {
		return ErrUnauthenticated
	}
	if !HasPermission(principal.Role, permission) {
		return ErrForbidden
	}
	return nil
}

// AuthorizeOwnProfile permits each active user to read their own profile only.
func AuthorizeOwnProfile(principal Principal, requestedUserID string) error {
	if principal.UserID == "" || !principal.IsActive || !IsRole(principal.Role) {
		return ErrUnauthenticated
	}
	if requestedUserID != principal.UserID {
		return ErrForbidden
	}
	return nil
}

// AuthorizeOwnedResource scopes Operator actions to resources created or assigned to them.
func AuthorizeOwnedResource(principal Principal, permission Permission, ownerID string) error {
	if err := Authorize(principal, permission); err != nil {
		return err
	}
	if principal.Role == RoleOperator && principal.UserID != ownerID {
		return ErrForbidden
	}
	return nil
}

// AuthorizeApproval enforces the independent Checker rule; Admin has no approval bypass.
func AuthorizeApproval(principal Principal, makerID string, permission Permission) error {
	if permission != PermissionMatchesApprove && permission != PermissionBatchesApprove && permission != PermissionBatchesClose {
		return ErrForbidden
	}
	if err := Authorize(principal, permission); err != nil {
		return err
	}
	if principal.Role != RoleChecker {
		return ErrForbidden
	}
	if makerID == "" || principal.UserID == makerID {
		return ErrSelfApproval
	}
	return nil
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}

// RequirePermission is the route guard for business endpoints once session auth is wired.
func RequirePermission(permission Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "Login diperlukan.")
			return
		}
		if err := Authorize(principal, permission); err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				httpx.Error(w, r, http.StatusUnauthorized, "unauthenticated", "Sesi tidak sah atau sudah tamat.")
				return
			}
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "Akses tidak diizinkan.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

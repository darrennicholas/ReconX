package exceptions

import (
	"errors"
	"reconx/backend/internal/modules/reconciliation"
	"testing"
)

func TestInvestigationAccessScopeAndState(t *testing.T) {
	tests := []struct {
		name, actor, owner, assigned, state, batchState, action string
		want                                                    error
	}{
		{"owner can assign unassigned", "owner", "owner", "", "OPEN", "REVIEW", "ASSIGN", nil},
		{"outsider cannot claim unassigned", "other", "owner", "", "OPEN", "REVIEW", "ASSIGN", reconciliation.ErrForbiddenScope},
		{"outsider cannot write evidence", "other", "owner", "", "OPEN", "REVIEW", "EVIDENCE", reconciliation.ErrForbiddenScope},
		{"assignee can investigate", "assigned", "owner", "assigned", "INVESTIGATING", "REVIEW", "NOTE", nil},
		{"owner can investigate assigned", "owner", "owner", "assigned", "INVESTIGATING", "REVIEW", "NOTE", nil},
		{"cannot reassign while approval pending", "owner", "owner", "assigned", "PENDING_ACTION_APPROVAL", "REVIEW", "ASSIGN", ErrApprovalConflict},
		{"note preserves pending approval", "assigned", "owner", "assigned", "PENDING_ACTION_APPROVAL", "REVIEW", "NOTE", nil},
		{"closed batch is immutable", "owner", "owner", "", "OPEN", "CLOSED", "NOTE", ErrApprovalConflict},
		{"archived exception is immutable", "owner", "owner", "", "ARCHIVED", "REVIEW", "EVIDENCE", ErrApprovalConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkInvestigationAccess(tt.actor, tt.owner, tt.assigned, tt.state, tt.batchState, tt.action)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v; want %v", err, tt.want)
			}
		})
	}
}

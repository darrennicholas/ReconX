package reconciliation

import "time"

// Batch represents one account and one reconciliation date period.
type Batch struct {
	ID            string
	AccountID     string
	PeriodStart   time.Time
	PeriodEnd     time.Time
	CreatedBy     string
	SourceVersion uint64
}

// Run records which source version produced a matching snapshot.
type Run struct {
	ID            string
	BatchID       string
	SourceVersion uint64
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// Match is a one-to-one pair between an internal transaction and a bank row.
type Match struct {
	ID                    string
	RunID                 string
	InternalTransactionID string
	BankTransactionID     string
	RuleID                string
	ManualMatchApprovalID string
}

type ApprovalType string

const (
	ApprovalManualMatch ApprovalType = "MANUAL_MATCH"
	ApprovalBatchClose  ApprovalType = "BATCH_CLOSE"
)

type ApprovalDecision string

const (
	ApprovalPending  ApprovalDecision = "PENDING"
	ApprovalApproved ApprovalDecision = "APPROVED"
	ApprovalRejected ApprovalDecision = "REJECTED"
)

// ManualMatchProposal remains a proposal until a Checker approves it.
type ManualMatchProposal struct {
	ID                    string
	BatchID               string
	SourceVersion         uint64
	InternalTransactionID string
	BankTransactionID     string
	MakerID               string
	Reason                string
	CreatedAt             time.Time
}

// Approval stores maker/checker identities separately for segregation of duties.
type Approval struct {
	ID            string
	BatchID       string
	Type          ApprovalType
	Decision      ApprovalDecision
	MakerID       string
	CheckerID     string
	SourceVersion uint64
	DecidedAt     *time.Time
}

// Close records the approved run snapshot that was locked by a Checker.
type Close struct {
	BatchID       string
	RunID         string
	SourceVersion uint64
	ApprovedBy    string
	ClosedAt      time.Time
}

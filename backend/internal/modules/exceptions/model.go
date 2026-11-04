package exceptions

type Type string

const (
	AmountMismatch Type = "AMOUNT_MISMATCH"
	InternalOnly   Type = "INTERNAL_ONLY"
	BankOnly       Type = "BANK_ONLY"
)

// Exception links the source rows that require investigation; IDs may be empty for one-sided records.
type Exception struct {
	ID                    string
	BatchID               string
	RunID                 string
	Type                  Type
	InternalTransactionID string
	BankTransactionID     string
	AssignedTo            string
}

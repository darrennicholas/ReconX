package reconciliation

import (
	"testing"
	"time"
)

func TestImportedDuplicateReferencesBecomeAmbiguous(t *testing.T) {
	date := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	internal, err := ParseImportCSV([]byte("reference_id,date,direction,amount,currency\nAMB-001,2026-11-01,CREDIT,100000,IDR\namb-001,2026-11-01,CREDIT,100000,IDR\n"), date, date)
	if err != nil || len(internal.Errors) != 0 || len(internal.Transactions) != 2 {
		t.Fatalf("duplicate import: transactions=%d errors=%v err=%v", len(internal.Transactions), internal.Errors, err)
	}
	rows := []TransactionView{
		{ID: "internal-1", ReferenceCanonical: internal.Transactions[0].ReferenceCanonical},
		{ID: "internal-2", ReferenceCanonical: internal.Transactions[1].ReferenceCanonical},
	}
	matches, exceptions := classify(rows, []TransactionView{{ID: "bank-1", ReferenceCanonical: "AMB-001"}})
	if len(matches) != 0 || len(exceptions) != 1 || exceptions[0].kind != "AMBIGUOUS_MATCH" || len(exceptions[0].candidateTransactionIDs) != 3 {
		t.Fatalf("unexpected matching result: matches=%v exceptions=%+v", matches, exceptions)
	}
}

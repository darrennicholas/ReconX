package reconciliation

import (
	"testing"
	"time"
)

func tx(id, ref, direction string, amount int64, day string) TransactionView {
	date, _ := time.Parse("2006-01-02", day)
	return TransactionView{ID: id, ReferenceCanonical: ref, Direction: direction, Amount: amount, Currency: "IDR", Date: date}
}

func TestClassifyFSDScenarios(t *testing.T) {
	tests := []struct {
		name           string
		internal, bank []TransactionView
		matched        int
		exceptionTypes map[string]int
	}{
		{
			name:     "FSD example has one exact pair and three exceptions",
			internal: []TransactionView{tx("i1", "TRX001", "CREDIT", 500000, "2026-11-03"), tx("i2", "TRX002", "CREDIT", 750000, "2026-11-03"), tx("i3", "TRX003", "CREDIT", 250000, "2026-11-03")},
			bank:     []TransactionView{tx("b1", "TRX001", "CREDIT", 500000, "2026-11-03"), tx("b2", "TRX002", "CREDIT", 700000, "2026-11-03"), tx("b4", "TRX004", "CREDIT", 300000, "2026-11-03")},
			matched:  1, exceptionTypes: map[string]int{"AMOUNT_MISMATCH": 1, "INTERNAL_ONLY": 1, "BANK_ONLY": 1},
		},
		{
			name:     "same amount with different reference never matches",
			internal: []TransactionView{tx("i1", "A", "CREDIT", 250000, "2026-11-03")},
			bank:     []TransactionView{tx("b1", "B", "CREDIT", 250000, "2026-11-03")},
			matched:  0, exceptionTypes: map[string]int{"INTERNAL_ONLY": 1, "BANK_ONLY": 1},
		},
		{
			name:     "duplicate reference candidates are ambiguous",
			internal: []TransactionView{tx("i1", "A", "CREDIT", 250000, "2026-11-03"), tx("i2", "A", "CREDIT", 250000, "2026-11-03")},
			bank:     []TransactionView{tx("b1", "A", "CREDIT", 250000, "2026-11-03")},
			matched:  0, exceptionTypes: map[string]int{"AMBIGUOUS_MATCH": 1},
		},
		{
			name:     "date variance is not an automatic match",
			internal: []TransactionView{tx("i1", "A", "CREDIT", 250000, "2026-11-03")},
			bank:     []TransactionView{tx("b1", "A", "CREDIT", 250000, "2026-11-04")},
			matched:  0, exceptionTypes: map[string]int{"DATE_VARIANCE": 1},
		},
		{
			name:     "direction mismatch is not an automatic match",
			internal: []TransactionView{tx("i1", "A", "CREDIT", 250000, "2026-11-03")},
			bank:     []TransactionView{tx("b1", "A", "DEBIT", 250000, "2026-11-03")},
			matched:  0, exceptionTypes: map[string]int{"DIRECTION_MISMATCH": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, exceptions := classify(tt.internal, tt.bank)
			if len(matches) != tt.matched {
				t.Fatalf("matched %d pairs, want %d", len(matches), tt.matched)
			}
			got := make(map[string]int)
			for _, exception := range exceptions {
				got[exception.kind]++
			}
			if len(got) != len(tt.exceptionTypes) {
				t.Fatalf("exception types %v, want %v", got, tt.exceptionTypes)
			}
			for kind, want := range tt.exceptionTypes {
				if got[kind] != want {
					t.Errorf("%s count %d, want %d", kind, got[kind], want)
				}
			}
		})
	}
}

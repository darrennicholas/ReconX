package reconciliation

import (
	"strings"
	"testing"
	"time"
)

func TestImportValidationAcceptance(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 30)
	header := "reference_id,date,direction,amount,currency\n"
	cases := []struct {
		name, csv, field string
		headerError      bool
	}{
		{"missing header", "reference_id,date\nA,2026-01-01\n", "", true},
		{"invalid amount", header + "A,2026-01-01,DEBIT,1.5,IDR\n", "amount", false},
		{"outside batch period", header + "A,2026-02-01,DEBIT,10,IDR\n", "date", false},
		{"invalid direction", header + "A,2026-01-01,UNKNOWN,10,IDR\n", "direction", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseImportCSV([]byte(tc.csv), start, end)
			if tc.headerError {
				if err == nil {
					t.Fatal("expected invalid header error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Errors) == 0 {
				t.Fatal("expected a row-level validation error")
			}
			found := false
			for _, issue := range result.Errors {
				if strings.EqualFold(issue.Field, tc.field) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s error: %+v", tc.field, result.Errors)
			}
		})
	}
}

package exceptions

import (
	"testing"
	"time"
)

func TestWithinCalendarDays(t *testing.T) {
	parse := func(value string) time.Time {
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			t.Fatal(err)
		}
		return date
	}
	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "three days forward is eligible", left: "2026-11-04", right: "2026-11-01", want: true},
		{name: "three days backward is eligible", left: "2026-11-01", right: "2026-11-04", want: true},
		{name: "four days is not eligible", left: "2026-11-05", right: "2026-11-01", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinCalendarDays(parse(tt.left), parse(tt.right), 3); got != tt.want {
				t.Fatalf("withinCalendarDays() = %t, want %t", got, tt.want)
			}
		})
	}
}

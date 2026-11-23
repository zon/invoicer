package invoice_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zon/invoicer/internal/invoice"
)

func TestWeeksForMonth_FullWeeks(t *testing.T) {
	// February 2024: Feb 1 is Thursday, Feb 29 is Thursday (leap year).
	// First Wednesday is Feb 7. Last Wednesday is Feb 28.
	weeks := invoice.WeeksForMonth(2024, time.February, 40)

	if len(weeks) == 0 {
		t.Fatal("expected at least one week")
	}

	// All weeks should have Wednesdays in February 2024.
	for i, w := range weeks {
		// Wednesday of this week
		wed := w.Start
		for wed.Weekday() != time.Wednesday {
			wed = wed.AddDate(0, 0, 1)
		}
		if wed.Month() != time.February || wed.Year() != 2024 {
			t.Errorf("week %d: Wednesday %v not in February 2024", i, wed)
		}
	}
}

func TestWeeksForMonth_WednesdayRule(t *testing.T) {
	// January 2025: Jan 1 is Wednesday.
	// The week with Wednesday Jan 1 should be included; its Monday is Dec 30, 2024.
	weeks := invoice.WeeksForMonth(2025, time.January, 40)

	if len(weeks) == 0 {
		t.Fatal("expected at least one week")
	}

	first := weeks[0]
	wantStart := time.Date(2024, time.December, 30, 0, 0, 0, 0, time.UTC)
	if !first.Start.Equal(wantStart) {
		t.Errorf("expected first week to start %v, got %v", wantStart, first.Start)
	}
}

func TestWeeksForMonth_ExcludesAdjacentMonthWednesdays(t *testing.T) {
	// March 2025: Mar 1 is Saturday. First Wednesday is Mar 5.
	// The week containing Wed Feb 26 should NOT be in March.
	weeks := invoice.WeeksForMonth(2025, time.March, 40)

	for i, w := range weeks {
		// Find Wednesday of this week.
		wed := w.Start
		for wed.Weekday() != time.Wednesday {
			wed = wed.AddDate(0, 0, 1)
		}
		if wed.Month() != time.March {
			t.Errorf("week %d: Wednesday %v is not in March 2025", i, wed)
		}
	}
}

func TestWeeksForMonth_FullHoursEveryWeek(t *testing.T) {
	// Every Wednesday-anchored week is billed at the full hoursPerWeek,
	// regardless of how the Mon-Sun span overlaps the month.
	weeks := invoice.WeeksForMonth(2025, time.January, 40)
	if len(weeks) == 0 {
		t.Fatal("expected weeks")
	}
	for i, w := range weeks {
		if w.Hours != 40.0 {
			t.Errorf("week %d: expected 40.0 hours, got %.1f (%v - %v)", i, w.Hours, w.Start, w.End)
		}
	}
}

func TestWeeksForMonth_AprilTotal(t *testing.T) {
	// April 2026 has 5 Wednesdays (1, 8, 15, 22, 29) → 5 weeks × 20h = 100h.
	weeks := invoice.WeeksForMonth(2026, time.April, 20)
	if len(weeks) != 5 {
		t.Fatalf("expected 5 weeks, got %d", len(weeks))
	}
	var total float64
	for _, w := range weeks {
		total += w.Hours
	}
	if total != 100.0 {
		t.Errorf("expected 100.0 total hours, got %.1f", total)
	}
}

func TestParseMonth_Numeric(t *testing.T) {
	tests := []struct {
		input string
		want  time.Month
	}{
		{"1", time.January},
		{"6", time.June},
		{"12", time.December},
	}
	for _, tt := range tests {
		got, err := invoice.ParseMonth(tt.input)
		if err != nil {
			t.Errorf("ParseMonth(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseMonth(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseMonth_TextFull(t *testing.T) {
	tests := []struct {
		input string
		want  time.Month
	}{
		{"January", time.January},
		{"february", time.February},
		{"MARCH", time.March},
		{"december", time.December},
	}
	for _, tt := range tests {
		got, err := invoice.ParseMonth(tt.input)
		if err != nil {
			t.Errorf("ParseMonth(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseMonth(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseMonth_TextAbbrev(t *testing.T) {
	tests := []struct {
		input string
		want  time.Month
	}{
		{"jan", time.January},
		{"Feb", time.February},
		{"DEC", time.December},
	}
	for _, tt := range tests {
		got, err := invoice.ParseMonth(tt.input)
		if err != nil {
			t.Errorf("ParseMonth(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseMonth(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseMonth_Invalid(t *testing.T) {
	for _, s := range []string{"0", "13", "foo", "jan2", "Month"} {
		_, err := invoice.ParseMonth(s)
		if err == nil {
			t.Errorf("ParseMonth(%q): expected error, got nil", s)
		}
	}
}

func TestResolveMonthYear_DefaultsPreviousMonth(t *testing.T) {
	now := time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.February {
		t.Errorf("expected February, got %v", month)
	}
	if year != 2025 {
		t.Errorf("expected 2025, got %d", year)
	}
}

func TestResolveMonthYear_DefaultsPreviousMonthAcrossYear(t *testing.T) {
	// January → previous month should be December of prior year.
	now := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.December {
		t.Errorf("expected December, got %v", month)
	}
	if year != 2024 {
		t.Errorf("expected 2024, got %d", year)
	}
}

func TestResolveMonthYear_ExplicitMonth(t *testing.T) {
	now := time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("june", 2024, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.June {
		t.Errorf("expected June, got %v", month)
	}
	if year != 2024 {
		t.Errorf("expected 2024, got %d", year)
	}
}

func TestResolveMonthYear_ExplicitMonthDefaultsCurrentYear(t *testing.T) {
	// An explicit month without a year uses the current year, even when that
	// month is ahead of today.
	now := time.Date(2025, time.November, 15, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("may", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.May {
		t.Errorf("expected May, got %v", month)
	}
	if year != 2025 {
		t.Errorf("expected 2025, got %d", year)
	}
}

func TestResolveMonthYear_DefaultsPreviousMonthOnShortMonth(t *testing.T) {
	// March 31 → previous month is February, not the normalized March 3.
	now := time.Date(2025, time.March, 31, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.February {
		t.Errorf("expected February, got %v", month)
	}
	if year != 2025 {
		t.Errorf("expected 2025, got %d", year)
	}
}

func TestInvoiceTotal(t *testing.T) {
	inv := invoice.Invoice{
		Rate: 100.0,
		Weeks: []invoice.Week{
			{Hours: 40},
			{Hours: 32},
			{Hours: 24},
		},
	}
	want := 9600.0
	if got := inv.Total(); got != want {
		t.Errorf("Total() = %.2f, want %.2f", got, want)
	}
}

func TestFormatWeekLabel_SameMonth(t *testing.T) {
	w := invoice.Week{
		Start: time.Date(2025, time.January, 6, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, time.January, 12, 0, 0, 0, 0, time.UTC),
	}
	got := invoice.FormatWeekLabel(w)
	// Should contain "Jan" and "6" and "12".
	if !strings.Contains(got, "Jan") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Jan'", got)
	}
	if !strings.Contains(got, "6") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain '6'", got)
	}
	if !strings.Contains(got, "12") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain '12'", got)
	}
}

func TestFormatWeekLabel_CrossMonth(t *testing.T) {
	w := invoice.Week{
		Start: time.Date(2025, time.January, 30, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, time.February, 2, 0, 0, 0, 0, time.UTC),
	}
	got := invoice.FormatWeekLabel(w)
	if !strings.Contains(got, "Jan") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Jan'", got)
	}
	if !strings.Contains(got, "Feb") {
		t.Errorf("FormatWeekLabel() = %q, expected to contain 'Feb'", got)
	}
}

package invoice_test

import (
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
	// The week with Wednesday Jan 1 should be included.
	weeks := invoice.WeeksForMonth(2025, time.January, 40)

	if len(weeks) == 0 {
		t.Fatal("expected at least one week")
	}

	// First week's start should be Jan 1 (clamped from Monday Dec 30).
	first := weeks[0]
	if first.Start.Day() != 1 || first.Start.Month() != time.January {
		t.Errorf("expected first week to start Jan 1, got %v", first.Start)
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

func TestWeeksForMonth_HoursProrated(t *testing.T) {
	// Choose a month where the first or last week is partial.
	// January 2025: Jan 1 is Wednesday. The week Mon Dec 30 - Sun Jan 5 has Wednesday Jan 1 in January.
	// But the week is clamped to Jan 1 - Jan 5 (5 workdays Mon-Fri: but Mon/Tue are in Dec).
	// Jan 1 (Wed), Jan 2 (Thu), Jan 3 (Fri) = 3 workdays out of 5 → 24 hours for 40h/week.
	weeks := invoice.WeeksForMonth(2025, time.January, 40)
	if len(weeks) == 0 {
		t.Fatal("expected weeks")
	}
	first := weeks[0]
	// Jan 1 (Wed), Jan 2 (Thu), Jan 3 (Fri) = 3 workdays
	expectedHours := 40.0 * 3 / 5
	if first.Hours != expectedHours {
		t.Errorf("expected first week hours %.1f, got %.1f", expectedHours, first.Hours)
	}
}

func TestWeeksForMonth_FullWeekHours(t *testing.T) {
	// A full Mon-Fri week should have exactly hoursPerWeek hours.
	// January 2025 week 2: Jan 6 (Mon) - Jan 12 (Sun), Wednesday Jan 8 is in January.
	weeks := invoice.WeeksForMonth(2025, time.January, 40)
	// Find a week that starts on Monday and ends on Sunday entirely within January.
	for _, w := range weeks {
		if w.Start.Weekday() == time.Monday && w.End.Weekday() == time.Sunday {
			if w.Hours != 40.0 {
				t.Errorf("expected full week hours 40.0, got %.1f (week %v - %v)", w.Hours, w.Start, w.End)
			}
			return
		}
	}
	// If no fully-unclamped week was found in January 2025, skip.
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

func TestResolveMonthYear_ClosestYear(t *testing.T) {
	// If month is given but not year, pick the closest year.
	// Today is March 2025. "November" without a year: Nov 2024 is ~4 months ago,
	// Nov 2025 is ~8 months away → 2024 is closer.
	now := time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC)
	month, year, err := invoice.ResolveMonthYear("november", 0, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if month != time.November {
		t.Errorf("expected November, got %v", month)
	}
	if year != 2024 {
		t.Errorf("expected 2024 (closest), got %d", year)
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

// Package invoice provides logic for generating invoices.
package invoice

import (
	"fmt"
	"time"
)

// Week represents a single weekly line item in an invoice.
type Week struct {
	// Start is the Monday of the week.
	Start time.Time
	// End is the Sunday of the week.
	End time.Time
	// Hours is the number of hours worked this week.
	Hours float64
}

// Invoice holds all data needed to generate an invoice for one calendar month.
type Invoice struct {
	// ID is the store's identifier for the invoice. It is empty until the
	// invoice is saved.
	ID string
	// Month is the calendar month (1-12).
	Month time.Month
	// Year is the calendar year.
	Year int
	// Vendor is the name of the contractor sending the invoice.
	Vendor string
	// Customer is the name of the customer receiving the invoice.
	Customer string
	// ContactEmail is the email address that receives the invoice.
	ContactEmail string
	// Project is the optional name of the project this invoice is for.
	Project string
	// Rate is the hourly rate in dollars.
	Rate float64
	// Paid reports whether the customer has paid the invoice.
	Paid bool
	// EmailSentAt is when the invoice email was sent. It is zero until a send
	// succeeds.
	EmailSentAt time.Time
	// Weeks is the list of weekly line items.
	Weeks []Week
}

// Total returns the total invoice amount.
func (inv *Invoice) Total() float64 {
	var total float64
	for _, w := range inv.Weeks {
		total += w.Hours * inv.Rate
	}
	return total
}

// WeeksForMonth returns the weeks that belong to the given month.
// A week belongs to a month if its Wednesday falls in that month.
// Weeks run Monday through Sunday and are billed at the full hoursPerWeek.
func WeeksForMonth(year int, month time.Month, hoursPerWeek float64) []Week {
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	wd := firstDay.Weekday()
	var daysToFirstWed int
	if wd <= time.Wednesday {
		daysToFirstWed = int(time.Wednesday - wd)
	} else {
		daysToFirstWed = int(7 - wd + time.Wednesday)
	}
	firstWed := firstDay.AddDate(0, 0, daysToFirstWed)

	var weeks []Week
	for wed := firstWed; !wed.After(lastDay); wed = wed.AddDate(0, 0, 7) {
		weeks = append(weeks, Week{
			Start: wed.AddDate(0, 0, -2),
			End:   wed.AddDate(0, 0, 4),
			Hours: hoursPerWeek,
		})
	}

	return weeks
}

// ParseMonth parses a month string (text or numeric) and returns the time.Month value.
// Returns an error if the string is not a valid month.
func ParseMonth(s string) (time.Month, error) {
	// Try numeric first.
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		if n < 1 || n > 12 {
			return 0, fmt.Errorf("month number %d out of range (1-12)", n)
		}
		return time.Month(n), nil
	}

	// Try text (full name or abbreviation).
	for m := time.January; m <= time.December; m++ {
		name := m.String()
		// Full match (case-insensitive).
		if equalFold(s, name) {
			return m, nil
		}
		// 3-letter abbreviation match.
		if len(s) == 3 && equalFold(s, name[:3]) {
			return m, nil
		}
	}

	return 0, fmt.Errorf("unrecognized month %q", s)
}

// equalFold compares two strings case-insensitively.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ResolveMonthYear resolves the month and year to use for an invoice.
// If monthStr is empty, defaults to the previous month.
// If year is 0, defaults to the current year, unless the month is omitted and
// the previous month falls in the prior year.
func ResolveMonthYear(monthStr string, year int, now time.Time) (time.Month, int, error) {
	var month time.Month
	var err error

	if monthStr == "" {
		// Default to previous month. Truncate to the first of the month so that
		// a short previous month does not spill into the wrong month.
		prev := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
		month = prev.Month()
		if year == 0 {
			year = prev.Year()
		}
	} else {
		month, err = ParseMonth(monthStr)
		if err != nil {
			return 0, 0, err
		}
		if year == 0 {
			year = now.Year()
		}
	}

	return month, year, nil
}

// FormatWeekLabel returns a human-readable label for a week range.
func FormatWeekLabel(w Week) string {
	if w.Start.Month() == w.End.Month() {
		return fmt.Sprintf("%s %d-%d", w.Start.Month().String()[:3], w.Start.Day(), w.End.Day())
	}
	return fmt.Sprintf("%s %d - %s %d",
		w.Start.Month().String()[:3], w.Start.Day(),
		w.End.Month().String()[:3], w.End.Day())
}

// Now returns the current time. Exposed as a variable for testing.
var Now = func() time.Time {
	return time.Now()
}

// Package invoice provides logic for generating invoices.
package invoice

import (
	"fmt"
	"time"
)

// Week represents a single weekly line item in an invoice.
type Week struct {
	// Start is the Monday of the week (or first day of the month if month starts mid-week).
	Start time.Time
	// End is the Friday of the week (or last day of the month if month ends mid-week).
	End time.Time
	// Hours is the number of hours worked this week.
	Hours float64
}

// Invoice holds all data needed to generate an invoice for one calendar month.
type Invoice struct {
	// Month is the calendar month (1-12).
	Month time.Month
	// Year is the calendar year.
	Year int
	// Vendor is the name of the contractor sending the invoice.
	Vendor string
	// Customer is the name of the client receiving the invoice.
	Customer string
	// Rate is the hourly rate in dollars.
	Rate float64
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
// Weeks run Monday through Sunday.
func WeeksForMonth(year int, month time.Month, hoursPerWeek float64) []Week {
	// Find the first Wednesday in or after the 1st of the month.
	// We iterate through the weeks whose Wednesday falls in the given month.
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	var weeks []Week

	// Find the first Wednesday >= firstDay of month.
	// Then walk backwards to find the Monday of that week.
	// Step through Wednesdays while they remain in the target month.

	// Find day offset to first Wednesday.
	wd := firstDay.Weekday()
	var daysToFirstWed int
	if wd <= time.Wednesday {
		daysToFirstWed = int(time.Wednesday - wd)
	} else {
		daysToFirstWed = int(7 - wd + time.Wednesday)
	}

	firstWed := firstDay.AddDate(0, 0, daysToFirstWed)

	for wed := firstWed; !wed.After(lastDay); wed = wed.AddDate(0, 0, 7) {
		// Monday of this week
		monday := wed.AddDate(0, 0, -2)
		// Sunday of this week
		sunday := wed.AddDate(0, 0, 4)

		// Clamp to month boundaries for display purposes
		weekStart := monday
		if weekStart.Before(firstDay) {
			weekStart = firstDay
		}
		weekEnd := sunday
		if weekEnd.After(lastDay) {
			weekEnd = lastDay
		}

		// Calculate prorated hours based on actual workdays (Mon-Fri) in the clamped range.
		workdays := countWorkdays(weekStart, weekEnd)
		hours := hoursPerWeek * float64(workdays) / 5.0

		weeks = append(weeks, Week{
			Start: weekStart,
			End:   weekEnd,
			Hours: hours,
		})
	}

	return weeks
}

// countWorkdays counts Monday-Friday days between start and end (inclusive).
func countWorkdays(start, end time.Time) int {
	count := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		wd := d.Weekday()
		if wd >= time.Monday && wd <= time.Friday {
			count++
		}
	}
	return count
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
// If year is 0, defaults to the year closest to the given month relative to today.
func ResolveMonthYear(monthStr string, year int, now time.Time) (time.Month, int, error) {
	var month time.Month
	var err error

	if monthStr == "" {
		// Default to previous month.
		prev := now.AddDate(0, -1, 0)
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
			// Choose the year closest to today for the given month.
			year = closestYear(month, now)
		}
	}

	return month, year, nil
}

// closestYear returns the year such that the given month is closest to now.
// It considers the current year, previous year, and next year.
func closestYear(month time.Month, now time.Time) int {
	best := now.Year()
	bestDiff := absDuration(now.Sub(time.Date(best, month, 1, 0, 0, 0, 0, time.UTC)))

	for _, y := range []int{now.Year() - 1, now.Year() + 1} {
		candidate := time.Date(y, month, 1, 0, 0, 0, 0, time.UTC)
		diff := absDuration(now.Sub(candidate))
		if diff < bestDiff {
			bestDiff = diff
			best = y
		}
	}

	return best
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

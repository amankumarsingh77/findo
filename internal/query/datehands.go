package query

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// handRulesParser handles well-known relative date phrases with exact,
// deterministic semantics. It must be tried before any third-party library
// so its outputs are bit-for-bit stable across library upgrades.
type handRulesParser struct{}

func (handRulesParser) Parse(lower string, now time.Time) (after, before time.Time, ok bool) {
	// Exact-phrase switch.
	switch lower {
	case "today":
		return startOfDay(now), now, true

	case "yesterday":
		y := now.AddDate(0, 0, -1)
		return startOfDay(y), endOfDay(y), true

	case "last week":
		return startOfDay(now.AddDate(0, 0, -7)), now, true

	case "last month":
		return startOfDay(now.AddDate(0, -1, 0)), now, true

	case "last year":
		return startOfDay(now.AddDate(-1, 0, 0)), now, true

	case "this morning":
		return atHour(now, 6), atHour(now, 12), true

	case "this afternoon":
		return atHour(now, 12), atHour(now, 18), true

	case "this evening":
		// 18:00 to 23:59:59
		return atHour(now, 18), atHour(now, 23).Add(time.Hour - time.Second), true

	case "past couple of months", "past couple months":
		return startOfDay(now.AddDate(0, -2, 0)), now, true

	case "past few months":
		return startOfDay(now.AddDate(0, -3, 0)), now, true

	case "last quarter":
		start, end := lastQuarterBounds(now)
		return start, end, true

	case "end of last quarter":
		_, end := lastQuarterBounds(now)
		return startOfDay(end), endOfDay(end), true
	}

	// "past N units".
	if a, b, k := parsePastN(lower, now); k {
		return a, b, true
	}

	// Year-only: "2025", "2024", etc.
	if yearOnlyRe.MatchString(lower) {
		y, _ := strconv.Atoi(lower)
		start := time.Date(y, 1, 1, 0, 0, 0, 0, now.Location())
		end := time.Date(y, 12, 31, 23, 59, 59, 0, now.Location())
		return start, end, true
	}

	return
}

// startOfDay returns midnight (00:00:00) on the same calendar day as t.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// endOfDay returns the last second (23:59:59) of the same calendar day as t.
func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// atHour returns the given hour (0-23) on the same calendar day as t.
func atHour(t time.Time, h int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), h, 0, 0, 0, t.Location())
}

// lastQuarterBounds returns [firstDay 00:00, lastDay 23:59:59] of the calendar
// quarter immediately before the quarter containing now.
func lastQuarterBounds(now time.Time) (time.Time, time.Time) {
	q := (int(now.Month())-1)/3 + 1 // 1..4
	lastQ := q - 1
	year := now.Year()
	if lastQ == 0 {
		lastQ = 4
		year--
	}
	startMonth := time.Month((lastQ-1)*3 + 1)
	start := time.Date(year, startMonth, 1, 0, 0, 0, 0, now.Location())
	// last day = first day of next quarter minus one day
	end := start.AddDate(0, 3, 0).AddDate(0, 0, -1)
	return start, endOfDay(end)
}

var pastNRe = regexp.MustCompile(`^past\s+(\d+)\s+(day|days|week|weeks|month|months|year|years)$`)
var yearOnlyRe = regexp.MustCompile(`^\d{4}$`)

func parsePastN(lower string, now time.Time) (after, before time.Time, ok bool) {
	m := pastNRe.FindStringSubmatch(lower)
	if m == nil {
		return
	}
	n, _ := strconv.Atoi(m[1])
	unit := m[2]
	var start time.Time
	switch {
	case strings.HasPrefix(unit, "day"):
		start = now.AddDate(0, 0, -n)
	case strings.HasPrefix(unit, "week"):
		start = now.AddDate(0, 0, -7*n)
	case strings.HasPrefix(unit, "month"):
		start = now.AddDate(0, -n, 0)
	case strings.HasPrefix(unit, "year"):
		start = now.AddDate(-n, 0, 0)
	default:
		return
	}
	return startOfDay(start), now, true
}

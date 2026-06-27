// Package daterange resolves named presets (e.g. "this-week") into concrete
// [start, end) windows for the Spending screen.
package daterange

import "time"

// Option is a selectable preset shown in the UI.
type Option struct{ Key, Label string }

var Options = []Option{
	{"this-week", "This Week"},
	{"last-week", "Last Week"},
	{"this-month", "This Month"},
	{"last-month", "Last Month"},
	{"last-7-days", "Last 7 Days"},
	{"last-30-days", "Last 30 Days"},
	{"last-90-days", "Last 90 Days"},
	{"this-year", "This Year"},
	{"last-12-months", "Last 12 Months"},
}

var Intervals = []string{"daily", "weekly", "monthly"}

// Resolve returns [start, end) for a named preset. Weeks start Monday.
// Unknown names fall back to last-30-days. now is injectable for testing.
func Resolve(name string, now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	loc := now.Location()
	today := time.Date(y, m, d, 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)

	weekday := (int(now.Weekday()) + 6) % 7 // Monday=0
	weekStart := today.AddDate(0, 0, -weekday)

	switch name {
	case "this-week":
		return weekStart, tomorrow
	case "last-week":
		return weekStart.AddDate(0, 0, -7), weekStart
	case "this-month":
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), tomorrow
	case "last-month":
		firstThis := time.Date(y, m, 1, 0, 0, 0, 0, loc)
		return firstThis.AddDate(0, -1, 0), firstThis
	case "last-7-days":
		return today.AddDate(0, 0, -6), tomorrow
	case "last-90-days":
		return today.AddDate(0, 0, -89), tomorrow
	case "this-year":
		return time.Date(y, 1, 1, 0, 0, 0, 0, loc), tomorrow
	case "last-12-months":
		return today.AddDate(-1, 0, 0), tomorrow
	case "last-30-days":
	}
	return today.AddDate(0, 0, -29), tomorrow // default
}

func Label(key string) string {
	for _, o := range Options {
		if o.Key == key {
			return o.Label
		}
	}
	return "Last 30 Days"
}

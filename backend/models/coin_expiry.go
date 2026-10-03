package models

import "time"

// CoinExpiry adds six calendar months, clamping the last day, in UTC.
func CoinExpiry(earned time.Time) time.Time {
	t := earned.UTC()
	month := time.Date(t.Year(), t.Month()+6, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	last := month.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return time.Date(month.Year(), month.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

package domain

import "time"

// ErrorFrequency counts error and fatal events in one UTC day.
type ErrorFrequency struct {
	Date   time.Time
	Errors int64
	Fatal  int64
}

// Total is the number of error and fatal events in the day.
func (f ErrorFrequency) Total() int64 {
	return f.Errors + f.Fatal
}

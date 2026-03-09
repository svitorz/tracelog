package domain

import "time"

// ErrorFrequency represents the number of errors in a specific day.
type ErrorFrequency struct {
	Date  time.Time
	Count int64
}

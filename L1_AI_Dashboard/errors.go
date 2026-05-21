package c2_dashboard

import (
	"errors"
	"time"
)

var (
	ErrInvalidTimeRange = errors.New("startTime must be before endTime")
	ErrDBUnavailable    = errors.New("database unavailable")
	ErrInternalError    = errors.New("internal query error")
)

func validateRequest(req DashboardQueryRequest) error {
	if req.StartTime != "" && req.EndTime != "" {
		t1, err1 := time.Parse(time.RFC3339, req.StartTime)
		t2, err2 := time.Parse(time.RFC3339, req.EndTime)
		if err1 != nil || err2 != nil {
			return ErrInvalidTimeRange
		}
		if t1.After(t2) {
			return ErrInvalidTimeRange
		}
	}
	return nil
}

func ClassifyError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrInvalidTimeRange):
		return 400, "INVALID_TIME_RANGE"
	case errors.Is(err, ErrDBUnavailable):
		return 503, "DB_UNAVAILABLE"
	default:
		return 500, "INTERNAL_ERROR"
	}
}

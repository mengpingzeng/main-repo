package workflow_engine

import (
	"errors"
	"time"
)

var (
	ErrInvalidTimeRange = errors.New("startTime must be before endTime")
	ErrDBUnavailable    = errors.New("database unavailable")
	ErrInternalError    = errors.New("internal query error")
)

func validateInput(input PublishInput) error {
	if input.SessionID == "" {
		return errors.New("sessionId is required")
	}
	if input.Platform == "" {
		return errors.New("platform is required")
	}
	if len(input.Accounts) == 0 {
		return errors.New("accounts is required")
	}
	return nil
}

func validateTimeRange(startTime, endTime string) error {
	if startTime != "" && endTime != "" {
		t1, err1 := time.Parse(time.RFC3339, startTime)
		t2, err2 := time.Parse(time.RFC3339, endTime)
		if err1 != nil || err2 != nil {
			return ErrInvalidTimeRange
		}
		if t1.After(t2) {
			return ErrInvalidTimeRange
		}
	}
	return nil
}

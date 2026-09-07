package leaderboard

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrSessionUnavailable = errors.New("session unavailable")
	ErrTooManySessions    = errors.New("too many concurrent active sessions; complete existing games first")
	ErrSessionUsed        = errors.New("session already used")
	ErrSessionExpired     = errors.New("session expired")
	ErrInvalidPayload     = errors.New("invalid score payload")
)

type InvalidPayloadError struct{ Message string }

func (e *InvalidPayloadError) Error() string { return e.Message }
func (e *InvalidPayloadError) Unwrap() error { return ErrInvalidPayload }

package domain

import (
	"errors"
	"fmt"
)

// Sentinels the adapters map onto HTTP status codes. Wrap them (%w) rather than returning
// them bare, so the log keeps the detail while callers can still match with errors.Is.
var (
	// ErrNotFound reports that a requested entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput reports user-supplied input that cannot be accepted. Prefer
	// returning a ValidationError, which carries a message safe to show the user.
	ErrInvalidInput = errors.New("invalid input")

	// ErrConflict reports input that collides with something already stored.
	ErrConflict = errors.New("conflict")

	// ErrFeedUnreachable reports a feed endpoint that could not be read or parsed.
	ErrFeedUnreachable = errors.New("feed unreachable")

	// ErrRateLimited reports that the provider asked us to slow down (HTTP 429). It
	// deliberately does not wrap ErrFeedUnreachable: being throttled says nothing about
	// whether the feed is valid, and the poller treats it as a skip rather than a failure.
	ErrRateLimited = errors.New("rate limited")
)

// Specific conflicts. Each wraps ErrConflict, so callers can match either the precise
// cause or the category.
var (
	ErrDuplicateURL   = fmt.Errorf("%w: this feed url is already subscribed", ErrConflict)
	ErrDuplicateTitle = fmt.Errorf("%w: this system name is already taken", ErrConflict)
)

// ValidationError is a rejected field, with a message written for the person who typed it.
// The HTTP adapter passes Message straight through to the client, so keep it free of
// internal detail.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Is reports ValidationError as an ErrInvalidInput, so callers can categorise it without
// unwrapping to the concrete type.
func (e ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

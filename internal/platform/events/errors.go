package events

import "errors"

// Sentinel errors for classifying consumer failures.
// Wrap with fmt.Errorf("context: %w", ErrPermanent) to signal the class.
var (
	ErrPermanent = errors.New("permanent error: will not retry")
	ErrTransient = errors.New("transient error: eligible for retry")
)

// IsPermanent returns true if the error should skip retries and go to DLQ immediately.
func IsPermanent(err error) bool {
	return errors.Is(err, ErrPermanent)
}

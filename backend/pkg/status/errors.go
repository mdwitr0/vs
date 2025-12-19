package status

import "errors"

var (
	// ErrInvalidTransition is returned when attempting an invalid status transition
	ErrInvalidTransition = errors.New("invalid status transition")

	// ErrConcurrentUpdate is returned when optimistic locking detects a concurrent modification
	ErrConcurrentUpdate = errors.New("concurrent update detected, document was modified by another process")

	// ErrInvalidStatus is returned when an unknown status value is encountered
	ErrInvalidStatus = errors.New("invalid status value")
)

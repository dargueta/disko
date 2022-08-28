package errors

import (
	"fmt"
)

// DriverError is a wrapper around system errno codes, with a customizable error message.
type DriverError interface {
	error
	Errno() Errno
	Unwrap() error
}

type driverError struct {
	errno         Errno
	message       string
	originalError error
}

// Error implements the `error` object interface. When called, it returns a string
// describing the error.
func (e driverError) Error() string {
	if e.message != "" {
		return e.message
	}
	return StrError(e.errno)
}

func (e driverError) Errno() Errno {
	return e.errno
}

func (e driverError) Unwrap() error {
	return e.originalError
}

// New creates a new [DriverError] with a default message derived from the
// system's error code.
func New(errnoCode Errno) DriverError {
	return driverError{
		errno:   errnoCode,
		message: StrError(errnoCode),
	}
}

func NewFromError(errnoCode Errno, originalError error) DriverError {
	return driverError{
		errno:         errnoCode,
		message:       fmt.Sprintf("%s: %s", StrError(errnoCode), originalError.Error()),
		originalError: originalError,
	}
}

// NewWithMessage creates a new DriverError from a system error code with a
// custom message.
func NewWithMessage(errnoCode Errno, message string) DriverError {
	return driverError{
		errno:   errnoCode,
		message: fmt.Sprintf("%s: %s", StrError(errnoCode), message),
	}
}

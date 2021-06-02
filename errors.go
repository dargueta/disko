package disko

import (
	"fmt"
)

// DriverError is a wrapper around system errno codes, with a customizable error message.
type DriverError struct {
	ErrnoCode Errno
	message   string
}

// Error implements the `error` object interface. When called, it returns a string
// describing the error.
func (e DriverError) Error() string {
	if e.message != "" {
		return e.message
	}
	return StrError(e.ErrnoCode)
}

// NewDriverError creates a new DriverError with a default message derived from the
// system's error code.
func NewDriverError(errnoCode Errno) *DriverError {
	return &DriverError{
		ErrnoCode: errnoCode,
		message:   StrError(errnoCode),
	}
}

// NewDriverErrorWithMessage creates a new DriverError from a system error code with a
// custom message.
func NewDriverErrorWithMessage(errnoCode Errno, message string) *DriverError {
	return &DriverError{
		ErrnoCode: errnoCode,
		message:   fmt.Sprintf("%s: %s", StrError(errnoCode), message),
	}
}

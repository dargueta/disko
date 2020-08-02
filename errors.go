package disko

import (
	"fmt"
	"syscall"
)

// DriverError is a wrapper around system errno codes, with a customizable error message.
type DriverError struct {
	ErrnoCode syscall.Errno
	message   string
}

// Error implements the `error` object interface. When called, it returns a string
// describing the error.
func (e DriverError) Error() string {
	if e.message != "" {
		return e.message
	}
	return e.ErrnoCode.Error()
}

// NewDriverError creates a new DriverError with a default message derived from the
// system's error code.
func NewDriverError(errnoCode syscall.Errno) *DriverError {
	return &DriverError{
		ErrnoCode: errnoCode,
		message:   errnoCode.Error(),
	}
}

// NewDriverErrorWithMessage creates a new DriverError from a system error code with a
// custom message.
func NewDriverErrorWithMessage(errnoCode syscall.Errno, message string) *DriverError {
	return &DriverError{
		ErrnoCode: errnoCode,
		message:   fmt.Sprintf("%s: %s", errnoCode.Error(), message),
	}
}

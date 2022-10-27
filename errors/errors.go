package errors

import "fmt"

type DriverError interface {
	error
	WithMessage(message string) DriverError
	WrapError(err error) DriverError
}

// -----------------------------------------------------------------------------

type customDriverError struct {
	message       string
	originalError error
}

// Error implements the `error` object interface. When called, it returns a string
// describing the error.
func (e customDriverError) Error() string {
	return e.message
}

func (e customDriverError) WithMessage(message string) DriverError {
	return customDriverError{
		message:       fmt.Sprintf("%s: %s", e.message, message),
		originalError: e,
	}
}

func (e customDriverError) WrapError(err error) DriverError {
	return customDriverError{
		message:       fmt.Sprintf("%s: %s", e.Error(), err.Error()),
		originalError: err,
	}
}

func (e customDriverError) Unwrap() error {
	return e.originalError
}

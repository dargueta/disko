package errors

type DriverError interface {
	error
	Errno() Errno
	Unwrap() error
	IsSameError(other error) bool
}

// -----------------------------------------------------------------------------

type driverErrorWithMessage struct {
	errno         Errno
	message       string
	originalError error
}

// Error implements the `error` object interface. When called, it returns a string
// describing the error.
func (e driverErrorWithMessage) Error() string {
	if e.message != "" {
		return e.message
	}
	return StrError(e.errno)
}

func (e driverErrorWithMessage) Errno() Errno {
	return e.errno
}

func (e driverErrorWithMessage) Unwrap() error {
	return e.originalError
}

func (e driverErrorWithMessage) IsSameError(other error) bool {
	driverError, ok := other.(DriverError)
	if ok {
		return e.errno == driverError.Errno()
	}
	return false
}

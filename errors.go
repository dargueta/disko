package disko

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

type DriverError interface {
	error
	WithMessage(message string) DriverError
	Wrap(err error) DriverError
}

type baseDiskoError string

const rootError = baseDiskoError("")

var ErrAlreadyInProgress = rootError.WithMessage("Operation already in progress")
var ErrArgumentOutOfRange = rootError.WithMessage("Numerical argument out of domain")
var ErrBlockDeviceRequired = rootError.WithMessage("Block device required")
var ErrBusy = rootError.WithMessage("Device or resource busy")
var ErrCrossDeviceLink = rootError.WithMessage("Invalid cross-device link")
var ErrDirectoryNotEmpty = rootError.WithMessage("Directory not empty")
var ErrDiskQuotaExceeded = rootError.WithMessage("Disk quota exceeded")
var ErrExists = rootError.WithMessage("File exists")
var ErrFileDescriptorBadState = rootError.WithMessage("File descriptor in bad state")
var ErrFileSystemCorrupted = rootError.WithMessage("Structure needs cleaning")
var ErrFileTooLarge = rootError.WithMessage("File too large")
var ErrInvalidArgument = rootError.WithMessage("Invalid argument")
var ErrInvalidFileDescriptor = rootError.WithMessage("Bad file descriptor")
var ErrInvalidFileSystem = rootError.WithMessage("Wrong medium type")
var ErrIOFailed = rootError.WithMessage("Input/output error")
var ErrIsADirectory = rootError.WithMessage("Is a directory")
var ErrLinkCycleDetected = rootError.WithMessage("Symlink cycle detected")
var ErrNameTooLong = rootError.WithMessage("File name too long")
var ErrNoDevice = rootError.WithMessage("No such device")
var ErrNoSpaceOnDevice = rootError.WithMessage("No space left on device")
var ErrNotADirectory = rootError.WithMessage("Not a directory")
var ErrNotFound = rootError.WithMessage("No such file or directory")
var ErrNotImplemented = rootError.WithMessage("Function not implemented")
var ErrNotPermitted = rootError.WithMessage("Operation not permitted")
var ErrNotSupported = rootError.WithMessage("Operation not supported")
var ErrPermissionDenied = rootError.WithMessage("Permission denied")
var ErrReadOnlyFileSystem = rootError.WithMessage("Read-only file system")
var ErrResultOutOfRange = rootError.WithMessage("Numerical result out of range")
var ErrTooManyLinks = rootError.WithMessage("Too many links")
var ErrTooManyOpenFiles = rootError.WithMessage("Too many open files in system")
var ErrTooManyUsers = rootError.WithMessage("Too many users")

func (e baseDiskoError) Error() string {
	return string(e)
}

func (e baseDiskoError) RootCause() DriverError {
	return e
}

func (e baseDiskoError) WithMessage(message string) DriverError {
	return customDriverError{
		message:       message,
		originalError: e,
	}
}

func (e baseDiskoError) Wrap(err error) DriverError {
	return customDriverError{
		message:       fmt.Sprintf("%s: %s", e.Error(), err.Error()),
		originalError: multierror.Append(e, err),
	}
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

func (e customDriverError) Wrap(err error) DriverError {
	return customDriverError{
		message:       fmt.Sprintf("%s: %s", e.Error(), err.Error()),
		originalError: multierror.Append(e, err),
	}
}

func (e customDriverError) Unwrap() error {
	return e.originalError
}

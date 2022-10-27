// This is a compatibility shim for POSIX-defined errno codes across platforms.
// The syscall package doesn't define all the values we need on all systems,
// particularly things like EUCLEAN.

package errors

import (
	"fmt"
)

type DiskoError string

const ErrAlreadyInProgress = DiskoError("Operation already in progress")
const ErrArgumentOutOfRange = DiskoError("Numerical argument out of domain")
const ErrBlockDeviceRequired = DiskoError("Block device required")
const ErrBusy = DiskoError("Device or resource busy")
const ErrCrossDeviceLink = DiskoError("Invalid cross-device link")
const ErrDirectoryNotEmpty = DiskoError("Directory not empty")
const ErrDiskQuotaExceeded = DiskoError("Disk quota exceeded")
const ErrExists = DiskoError("File exists")
const ErrFileDescriptorBadState = DiskoError("File descriptor in bad state")
const ErrFileSystemCorrupted = DiskoError("Structure needs cleaning")
const ErrFileTooLarge = DiskoError("File too large")
const ErrInvalidArgument = DiskoError("Invalid argument")
const ErrInvalidFileDescriptor = DiskoError("Bad file descriptor")
const ErrInvalidFileSystem = DiskoError("Wrong medium type")
const ErrIOFailed = DiskoError("Input/output error")
const ErrIsADirectory = DiskoError("Is a directory")
const ErrLinkCycleDetected = DiskoError("Symlink cycle detected")
const ErrNameTooLong = DiskoError("File name too long")
const ErrNoDevice = DiskoError("No such device")
const ErrNoSpaceOnDevice = DiskoError("No space left on device")
const ErrNotADirectory = DiskoError("Not a directory")
const ErrNotFound = DiskoError("No such file or directory")
const ErrNotImplemented = DiskoError("Function not implemented")
const ErrNotPermitted = DiskoError("Operation not permitted")
const ErrNotSupported = DiskoError("Operation not supported")
const ErrPermissionDenied = DiskoError("Permission denied")
const ErrReadOnlyFileSystem = DiskoError("Read-only file system")
const ErrResultOutOfRange = DiskoError("Numerical result out of range")
const ErrTooManyLinks = DiskoError("Too many links")
const ErrTooManyOpenFiles = DiskoError("Too many open files in system")
const ErrTooManyUsers = DiskoError("Too many users")
const ErrUnexpectedEOF = DiskoError("Unexpected end of file or stream")

func (e DiskoError) Error() string {
	return string(e)
}

func (e DiskoError) WithMessage(message string) DriverError {
	return customDriverError{
		message:       message,
		originalError: e,
	}
}

func (e DiskoError) WrapError(err error) DriverError {
	return customDriverError{
		message:       fmt.Sprintf("%s %s", e.Error(), err.Error()),
		originalError: err,
	}
}

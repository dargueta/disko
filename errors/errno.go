// This is a compatibility shim for POSIX-defined errno codes across platforms.
// The syscall package doesn't define all the values we need on all systems,
// particularly things like EUCLEAN.

package errors

import (
	"fmt"
)

type Errno int

var errorMessagesByCode map[Errno]string

const (
	_ Errno = iota
	ErrNotPermitted
	ErrNotFound
	ErrIOFailed
	ErrInvalidFileDescriptor
	ErrPermissionDenied
	ErrBlockDeviceRequired
	ErrBusy
	ErrExists
	ErrCrossDeviceLink
	ErrNoDevice
	ErrNotADirectory
	ErrIsADirectory
	ErrInvalidArgument
	ErrTooManyOpenFiles
	ErrFileTooLarge
	ErrNoSpaceOnDevice
	ErrReadOnlyFileSystem
	ErrTooManyLinks
	ErrArgumentOutOfRange
	ErrResultOutOfRange
	ErrNameTooLong
	ErrNotImplemented
	ErrDirectoryNotEmpty
	ErrLinkCycleDetected
	ErrFileDescriptorBadState
	ErrTooManyUsers
	ErrNotSupported
	ErrAlreadyInProgress
	ErrFileSystemCorrupted
	ErrDiskQuotaExceeded
	ErrInvalidFileSystem
	ErrUnexpectedEOF
	maxErrorCode
)

func init() {
	errorMessagesByCode = make(map[Errno]string, maxErrorCode)
	errorMessagesByCode[ErrNotPermitted] = "Operation not permitted"
	errorMessagesByCode[ErrNotFound] = "No such file or directory"
	errorMessagesByCode[ErrIOFailed] = "Input/output error"
	errorMessagesByCode[ErrInvalidFileDescriptor] = "Bad file descriptor"
	errorMessagesByCode[ErrPermissionDenied] = "Permission denied"
	errorMessagesByCode[ErrBlockDeviceRequired] = "Block device required"
	errorMessagesByCode[ErrBusy] = "Device or resource busy"
	errorMessagesByCode[ErrExists] = "File exists"
	errorMessagesByCode[ErrCrossDeviceLink] = "Invalid cross-device link"
	errorMessagesByCode[ErrNoDevice] = "No such device"
	errorMessagesByCode[ErrNotADirectory] = "Not a directory"
	errorMessagesByCode[ErrIsADirectory] = "Is a directory"
	errorMessagesByCode[ErrInvalidArgument] = "Invalid argument"
	errorMessagesByCode[ErrTooManyOpenFiles] = "Too many open files in system"
	errorMessagesByCode[ErrFileTooLarge] = "File too large"
	errorMessagesByCode[ErrNoSpaceOnDevice] = "No space left on device"
	errorMessagesByCode[ErrReadOnlyFileSystem] = "Read-only file system"
	errorMessagesByCode[ErrTooManyLinks] = "Too many links"
	errorMessagesByCode[ErrArgumentOutOfRange] = "Numerical argument out of domain"
	errorMessagesByCode[ErrResultOutOfRange] = "Numerical result out of range"
	errorMessagesByCode[ErrNameTooLong] = "File name too long"
	errorMessagesByCode[ErrNotImplemented] = "Function not implemented"
	errorMessagesByCode[ErrDirectoryNotEmpty] = "Directory not empty"
	errorMessagesByCode[ErrTooManyLinks] = "Too many levels of symbolic links"
	errorMessagesByCode[ErrFileDescriptorBadState] = "File descriptor in bad state"
	errorMessagesByCode[ErrTooManyUsers] = "Too many users"
	errorMessagesByCode[ErrNotSupported] = "Operation not supported"
	errorMessagesByCode[ErrAlreadyInProgress] = "Operation already in progress"
	errorMessagesByCode[ErrFileSystemCorrupted] = "Structure needs cleaning"
	errorMessagesByCode[ErrDiskQuotaExceeded] = "Disk quota exceeded"
	errorMessagesByCode[ErrInvalidFileSystem] = "Wrong medium type"
	errorMessagesByCode[ErrUnexpectedEOF] = "Unexpected end of file or stream"
}

func StrError(code Errno) string {
	message, ok := errorMessagesByCode[code]
	if ok {
		return message
	}
	return fmt.Sprintf("error %d not recognized.", int(code))
}

func (e Errno) Error() string {
	return StrError(e)
}

func (e Errno) Errno() Errno {
	return e
}

func (e Errno) Unwrap() error {
	return nil
}

func (e Errno) WithMessage(message string) DriverError {
	return driverErrorWithMessage{
		errno:         e,
		message:       message,
		originalError: nil,
	}
}

func (e Errno) WrapError(err error) DriverError {
	return driverErrorWithMessage{
		errno:         e,
		message:       fmt.Sprintf("error: [%d] %s", int(e), err.Error()),
		originalError: err,
	}
}

func (e Errno) IsSameError(other error) bool {
	driverError, ok := other.(DriverError)
	if ok {
		return e == driverError.Errno()
	}
	return false
}

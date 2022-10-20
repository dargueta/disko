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
	EOK Errno = iota
	EPERM
	ENOENT
	EINTR
	EIO
	EBADF
	EAGAIN
	EACCES
	EFAULT
	ENOTBLK
	EBUSY
	EEXIST
	EXDEV
	ENODEV
	ENOTDIR
	EISDIR
	EINVAL
	ENFILE
	EMFILE
	EFBIG
	ENOSPC
	ESPIPE
	EROFS
	EMLINK
	EDOM
	ERANGE
	EDEADLK
	ENAMETOOLONG
	ENOSYS
	ENOTEMPTY
	ELOOP
	ENODATA
	EOVERFLOW
	EBADFD
	EUSERS
	ENOTSUP
	ENOBUFS
	EALREADY
	ESTALE
	EUCLEAN
	EDQUOT
	EMEDIUMTYPE
)

var ErrNotPermitted = New(EPERM)
var ErrNotFound = New(ENOENT)
var ErrIOFailed = New(EIO)
var ErrInvalidFileDescriptor = New(EBADF)
var ErrBlockDeviceRequired = New(ENOTBLK)
var ErrBusy = New(EBUSY)
var ErrExists = New(EEXIST)
var ErrPermissionDenied = New(EACCES)
var ErrCrossDeviceLink = New(EXDEV)
var ErrNoDevice = New(ENODEV)
var ErrNotADirectory = New(ENOTDIR)
var ErrIsADirectory = New(EISDIR)
var ErrInvalidArgument = New(EINVAL)
var ErrTooManyOpenFiles = New(EMFILE)
var ErrFileTooLarge = New(EFBIG)
var ErrNoSpaceOnDevice = New(ENOSPC)
var ErrReadOnlyFileSystem = New(EROFS)
var ErrTooManyLinks = New(EMLINK)
var ErrArgumentOutOfRange = New(EDOM)
var ErrResultOutOfRange = New(ERANGE)
var ErrNameTooLong = New(ENAMETOOLONG)
var ErrNotImplemented = New(ENOSYS)
var ErrDirectoryNotEmpty = New(ENOTEMPTY)
var ErrLinkCycleDetected = New(ELOOP)
var ErrBrokenSymlink = NewWithMessage(ENOENT, "symlink is broken")
var ErrFileDescriptorBadState = New(EBADFD)
var ErrTooManyUsers = New(EUSERS)
var ErrNotSupported = New(ENOTSUP)
var ErrStaleFileHandle = New(ESTALE)
var ErrFileSystemCorrupted = New(EUCLEAN)
var ErrDiskQuotaExceeded = New(EDQUOT)
var ErrAlreadyInProgress = New(EALREADY)

func init() {
	errorMessagesByCode = make(map[Errno]string, 32)
	errorMessagesByCode[EPERM] = "Operation not permitted"
	errorMessagesByCode[ENOENT] = "No such file or directory"
	errorMessagesByCode[EINTR] = "Interrupted system call"
	errorMessagesByCode[EIO] = "Input/output error"
	errorMessagesByCode[EBADF] = "Bad file descriptor"
	errorMessagesByCode[EAGAIN] = "Resource temporarily unavailable"
	errorMessagesByCode[EACCES] = "Permission denied"
	errorMessagesByCode[EFAULT] = "Bad address"
	errorMessagesByCode[ENOTBLK] = "Block device required"
	errorMessagesByCode[EBUSY] = "Device or resource busy"
	errorMessagesByCode[EEXIST] = "File exists"
	errorMessagesByCode[EXDEV] = "Invalid cross-device link"
	errorMessagesByCode[ENODEV] = "No such device"
	errorMessagesByCode[ENOTDIR] = "Not a directory"
	errorMessagesByCode[EISDIR] = "Is a directory"
	errorMessagesByCode[EINVAL] = "Invalid argument"
	errorMessagesByCode[ENFILE] = "Too many open files in system"
	errorMessagesByCode[EMFILE] = "Too many open files"
	errorMessagesByCode[EFBIG] = "File too large"
	errorMessagesByCode[ENOSPC] = "No space left on device"
	errorMessagesByCode[ESPIPE] = "Illegal seek"
	errorMessagesByCode[EROFS] = "Read-only file system"
	errorMessagesByCode[EMLINK] = "Too many links"
	errorMessagesByCode[EDOM] = "Numerical argument out of domain"
	errorMessagesByCode[ERANGE] = "Numerical result out of range"
	errorMessagesByCode[ENAMETOOLONG] = "File name too long"
	errorMessagesByCode[ENOSYS] = "Function not implemented"
	errorMessagesByCode[ENOTEMPTY] = "Directory not empty"
	errorMessagesByCode[ELOOP] = "Too many levels of symbolic links"
	errorMessagesByCode[ENODATA] = "No data available"
	errorMessagesByCode[EOVERFLOW] = "Value too large for defined data type"
	errorMessagesByCode[EBADFD] = "File descriptor in bad state"
	errorMessagesByCode[EUSERS] = "Too many users"
	errorMessagesByCode[ENOTSUP] = "Operation not supported"
	errorMessagesByCode[ENOBUFS] = "No buffer space available"
	errorMessagesByCode[EALREADY] = "Operation already in progress"
	errorMessagesByCode[ESTALE] = "Stale file handle"
	errorMessagesByCode[EUCLEAN] = "Structure needs cleaning"
	errorMessagesByCode[EDQUOT] = "Disk quota exceeded"
	errorMessagesByCode[EMEDIUMTYPE] = "Wrong medium type"
}

func StrError(code Errno) string {
	message, ok := errorMessagesByCode[code]
	if ok {
		return message
	}
	return fmt.Sprintf("error %d not recognized.", int(code))
}

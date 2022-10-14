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
	ESRCH
	EINTR
	EIO
	ENXIO
	E2BIG
	ENOEXEC
	EBADF
	ECHILD
	EAGAIN
	EACCES
	ENOMEM
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
	ENOTTY
	ETXTBSY
	EFBIG
	ENOSPC
	ESPIPE
	EROFS
	EMLINK
	EPIPE
	EDOM
	ERANGE
	EDEADLK
	ENAMETOOLONG
	ENOLCK
	ENOSYS
	ENOTEMPTY
	ELOOP
	_ // 0x29 not assigned
	ENOMSG
	EIDRM
	ECHRNG
	EL2NSYNC
	EL3HLT
	EL3RST
	ELNRNG
	EUNATCH
	ENOCSI
	EL2HLT
	EBADE
	EBADR
	EXFULL
	ENOANO
	EBADRQC
	EBADSLT
	_ // 0x3a not assigned
	EBFONT
	ENOSTR
	ENODATA
	ETIME
	ENOSR
	ENONET
	ENOPKG
	EREMOTE
	ENOLINK
	EADV
	ESRMNT
	ECOMM
	EPROTO
	EMULTIHOP
	EDOTDOT
	EBADMSG
	EOVERFLOW
	ENOTUNIQ
	EBADFD
	EREMCHG
	ELIBACC
	ELIBBAD
	ELIBSCN
	ELIBMAX
	ELIBEXEC
	EILSEQ
	ERESTART
	ESTRPIPE
	EUSERS
	ENOTSOCK
	EDESTADDRREQ
	EMSGSIZE
	EPROTOTYPE
	ENOPROTOOPT
	EPROTONOSUPPORT
	ESOCKTNOSUPPORT
	ENOTSUP
	EPFNOSUPPORT
	EAFNOSUPPORT
	EADDRINUSE
	EADDRNOTAVAIL
	ENETDOWN
	ENETUNREACH
	ENETRESET
	ECONNABORTED
	ECONNRESET
	ENOBUFS
	EISCONN
	ENOTCONN
	ESHUTDOWN
	ETOOMANYREFS
	ETIMEDOUT
	ECONNREFUSED
	EHOSTDOWN
	EHOSTUNREACH
	EALREADY
	EINPROGRESS
	ESTALE
	EUCLEAN
	ENOTNAM
	ENAVAIL
	EISNAM
	EREMOTEIO
	EDQUOT
	ENOMEDIUM
	EMEDIUMTYPE
	ECANCELED
	ENOKEY
	EKEYEXPIRED
	EKEYREVOKED
	EKEYREJECTED
	EOWNERDEAD
	ENOTRECOVERABLE
	ERFKILL
)

// EWOULDBLOCK is a synonym for EAGAIN.
const EWOULDBLOCK = EAGAIN

// EDEADLOCK is a synonym for EDEADLK.
const EDEADLOCK = EDEADLK

// EOPNOTSUPP is a synonym for ENOTSUP
const EOPNOTSUPP = ENOTSUP

func init() {
	errorMessagesByCode = make(map[Errno]string, 32)
	errorMessagesByCode[EOK] = "Success"
	errorMessagesByCode[EPERM] = "Operation not permitted"
	errorMessagesByCode[ENOENT] = "No such file or directory"
	errorMessagesByCode[ESRCH] = "No such process"
	errorMessagesByCode[EINTR] = "Interrupted system call"
	errorMessagesByCode[EIO] = "Input/output error"
	errorMessagesByCode[ENXIO] = "No such device or address"
	errorMessagesByCode[E2BIG] = "Argument list too long"
	errorMessagesByCode[ENOEXEC] = "Exec format error"
	errorMessagesByCode[EBADF] = "Bad file descriptor"
	errorMessagesByCode[ECHILD] = "No child processes"
	errorMessagesByCode[EAGAIN] = "Resource temporarily unavailable"
	errorMessagesByCode[ENOMEM] = "Cannot allocate memory"
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
	errorMessagesByCode[ENOTTY] = "Inappropriate ioctl for device"
	errorMessagesByCode[ETXTBSY] = "Text file busy"
	errorMessagesByCode[EFBIG] = "File too large"
	errorMessagesByCode[ENOSPC] = "No space left on device"
	errorMessagesByCode[ESPIPE] = "Illegal seek"
	errorMessagesByCode[EROFS] = "Read-only file system"
	errorMessagesByCode[EMLINK] = "Too many links"
	errorMessagesByCode[EPIPE] = "Broken pipe"
	errorMessagesByCode[EDOM] = "Numerical argument out of domain"
	errorMessagesByCode[ERANGE] = "Numerical result out of range"
	errorMessagesByCode[EDEADLOCK] = "Resource deadlock avoided"
	errorMessagesByCode[ENAMETOOLONG] = "File name too long"
	errorMessagesByCode[ENOLCK] = "No locks available"
	errorMessagesByCode[ENOSYS] = "Function not implemented"
	errorMessagesByCode[ENOTEMPTY] = "Directory not empty"
	errorMessagesByCode[ELOOP] = "Too many levels of symbolic links"
	errorMessagesByCode[ENOMSG] = "No message of desired type"
	errorMessagesByCode[EIDRM] = "Identifier removed"
	errorMessagesByCode[ECHRNG] = "Channel number out of range"
	errorMessagesByCode[EL2NSYNC] = "Level 2 not synchronized"
	errorMessagesByCode[EL3HLT] = "Level 3 halted"
	errorMessagesByCode[EL3RST] = "Level 3 reset"
	errorMessagesByCode[ELNRNG] = "Link number out of range"
	errorMessagesByCode[EUNATCH] = "Protocol driver not attached"
	errorMessagesByCode[ENOCSI] = "No CSI structure available"
	errorMessagesByCode[EL2HLT] = "Level 2 halted"
	errorMessagesByCode[EBADE] = "Invalid exchange"
	errorMessagesByCode[EBADR] = "Invalid request descriptor"
	errorMessagesByCode[EXFULL] = "Exchange full"
	errorMessagesByCode[ENOANO] = "No anode"
	errorMessagesByCode[EBADRQC] = "Invalid request code"
	errorMessagesByCode[EBADSLT] = "Invalid slot"
	errorMessagesByCode[EBFONT] = "Bad font file format"
	errorMessagesByCode[ENOSTR] = "Device not a stream"
	errorMessagesByCode[ENODATA] = "No data available"
	errorMessagesByCode[ETIME] = "Timer expired"
	errorMessagesByCode[ENOSR] = "Out of streams resources"
	errorMessagesByCode[ENONET] = "Machine is not on the network"
	errorMessagesByCode[ENOPKG] = "Package not installed"
	errorMessagesByCode[EREMOTE] = "Object is remote"
	errorMessagesByCode[ENOLINK] = "Link has been severed"
	errorMessagesByCode[EADV] = "Advertise error"
	errorMessagesByCode[ESRMNT] = "Srmount error"
	errorMessagesByCode[ECOMM] = "Communication error on send"
	errorMessagesByCode[EPROTO] = "Protocol error"
	errorMessagesByCode[EMULTIHOP] = "Multihop attempted"
	errorMessagesByCode[EDOTDOT] = "RFS specific error"
	errorMessagesByCode[EBADMSG] = "Bad message"
	errorMessagesByCode[EOVERFLOW] = "Value too large for defined data type"
	errorMessagesByCode[ENOTUNIQ] = "Name not unique on network"
	errorMessagesByCode[EBADFD] = "File descriptor in bad state"
	errorMessagesByCode[EREMCHG] = "Remote address changed"
	errorMessagesByCode[ELIBACC] = "Can not access a needed shared library"
	errorMessagesByCode[ELIBBAD] = "Accessing a corrupted shared library"
	errorMessagesByCode[ELIBSCN] = ".lib section in a.out corrupted"
	errorMessagesByCode[ELIBMAX] = "Attempting to link in too many shared libraries"
	errorMessagesByCode[ELIBEXEC] = "Cannot exec a shared library directly"
	errorMessagesByCode[EILSEQ] = "Invalid or incomplete multibyte or wide character"
	errorMessagesByCode[ERESTART] = "Interrupted system call should be restarted"
	errorMessagesByCode[ESTRPIPE] = "Streams pipe error"
	errorMessagesByCode[EUSERS] = "Too many users"
	errorMessagesByCode[ENOTSOCK] = "Socket operation on non-socket"
	errorMessagesByCode[EDESTADDRREQ] = "Destination address required"
	errorMessagesByCode[EMSGSIZE] = "Message too long"
	errorMessagesByCode[EPROTOTYPE] = "Protocol wrong type for socket"
	errorMessagesByCode[ENOPROTOOPT] = "Protocol not available"
	errorMessagesByCode[EPROTONOSUPPORT] = "Protocol not supported"
	errorMessagesByCode[ESOCKTNOSUPPORT] = "Socket type not supported"
	errorMessagesByCode[ENOTSUP] = "Operation not supported"
	errorMessagesByCode[EPFNOSUPPORT] = "Protocol family not supported"
	errorMessagesByCode[EAFNOSUPPORT] = "Address family not supported by protocol"
	errorMessagesByCode[EADDRINUSE] = "Address already in use"
	errorMessagesByCode[EADDRNOTAVAIL] = "Cannot assign requested address"
	errorMessagesByCode[ENETDOWN] = "Network is down"
	errorMessagesByCode[ENETUNREACH] = "Network is unreachable"
	errorMessagesByCode[ENETRESET] = "Network dropped connection on reset"
	errorMessagesByCode[ECONNABORTED] = "Software caused connection abort"
	errorMessagesByCode[ECONNRESET] = "Connection reset by peer"
	errorMessagesByCode[ENOBUFS] = "No buffer space available"
	errorMessagesByCode[EISCONN] = "Transport endpoint is already connected"
	errorMessagesByCode[ENOTCONN] = "Transport endpoint is not connected"
	errorMessagesByCode[ESHUTDOWN] = "Cannot send after transport endpoint shutdown"
	errorMessagesByCode[ETOOMANYREFS] = "Too many references: cannot splice"
	errorMessagesByCode[ETIMEDOUT] = "Connection timed out"
	errorMessagesByCode[ECONNREFUSED] = "Connection refused"
	errorMessagesByCode[EHOSTDOWN] = "Host is down"
	errorMessagesByCode[EHOSTUNREACH] = "No route to host"
	errorMessagesByCode[EALREADY] = "Operation already in progress"
	errorMessagesByCode[EINPROGRESS] = "Operation now in progress"
	errorMessagesByCode[ESTALE] = "Stale file handle"
	errorMessagesByCode[EUCLEAN] = "Structure needs cleaning"
	errorMessagesByCode[ENOTNAM] = "Not a XENIX named type file"
	errorMessagesByCode[ENAVAIL] = "No XENIX semaphores available"
	errorMessagesByCode[EISNAM] = "Is a named type file"
	errorMessagesByCode[EREMOTEIO] = "Remote I/O error"
	errorMessagesByCode[EDQUOT] = "Disk quota exceeded"
	errorMessagesByCode[ENOMEDIUM] = "No medium found"
	errorMessagesByCode[EMEDIUMTYPE] = "Wrong medium type"
	errorMessagesByCode[ECANCELED] = "Operation canceled"
	errorMessagesByCode[ENOKEY] = "Required key not available"
	errorMessagesByCode[EKEYEXPIRED] = "Key has expired"
	errorMessagesByCode[EKEYREVOKED] = "Key has been revoked"
	errorMessagesByCode[EKEYREJECTED] = "Key was rejected by service"
	errorMessagesByCode[EOWNERDEAD] = "Owner died"
	errorMessagesByCode[ENOTRECOVERABLE] = "State not recoverable"
	errorMessagesByCode[ERFKILL] = "Operation not possible due to RF-kill"
}

func StrError(code Errno) string {
	message, ok := errorMessagesByCode[code]
	if ok {
		return message
	}
	return fmt.Sprintf("error %d not recognized.", int(code))
}

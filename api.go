package disko

import (
	"io"
	"os"
	"syscall"
	"time"
)

type MountFlags int

// TODO (dargueta): These permissions are too granular and don't make a lot of sense.
const (
	// MountFlagsAllowRead indicates to Driver.Mount() that the image should be
	// mounted with read permissions.
	MountFlagsAllowRead = MountFlags(1 << iota)
	// MountFlagsAllowWrite indicates to Driver.Mount() that the image should be
	// mounted with write permissions. Existing files can be modified, but
	// nothing can be created or deleted.
	MountFlagsAllowWrite = MountFlags(1 << iota)
	// MountFlagsAllowInsert indicates to Driver.Mount() that the image should
	// be mounted with insert permissions. New files and directories can be
	// created and modified, but existing files cannot be touched unless
	// MountFlagsAllowWrite is also specified.
	MountFlagsAllowInsert = MountFlags(1 << iota)
	// MountFlagsAllowDelete indicates to Driver.Mount() that the image should
	// be mounted with permissions to delete files and directories.
	MountFlagsAllowDelete = MountFlags(1 << iota)
	// MountFlagsAllowAdminister indicates to Driver.Mount() that the image
	// should be mounted with the ability to change file permissions.
	MountFlagsAllowAdminister = MountFlags(1 << iota)
	// MountFlagsPreserveTimestamps indicates that existing objects'
	// LastAccessed, LastModified, LastChanged, and DeletedAt timestamps should
	// NOT be changed. There are a few exceptions:
	//
	// Objects created or deleted will have their timestamps set appropriately
	// and then left alone for the duration of the mount.
	MountFlagsPreserveTimestamps = MountFlags(1 << iota)
	// MountFlagsCustomStart is the lowest bit flag that is not defined by the
	// API standard and is free for drivers to use in an implementation-specific
	// manner. All bits higher than this are guaranteed to be ignored by drivers
	// that do not recognize them.
	MountFlagsCustomStart = MountFlags(1 << iota)
)

func (flags MountFlags) CanRead() bool {
	return flags&MountFlagsAllowRead != 0
}

func (flags MountFlags) CanWrite() bool {
	return flags&MountFlagsAllowWrite != 0
}

func (flags MountFlags) CanDelete() bool {
	return flags&MountFlagsAllowDelete != 0
}

const MountFlagsAllowReadWrite = MountFlagsAllowRead | MountFlagsAllowWrite
const MountFlagsAllowAll = (MountFlagsAllowRead |
	MountFlagsAllowWrite |
	MountFlagsAllowInsert |
	MountFlagsAllowDelete |
	MountFlagsAllowAdminister)
const MountFlagsMask = MountFlagsCustomStart - 1

// FileStat is a platform-independent form of syscall.Stat_t.
type FileStat struct {
	DeviceID     uint64
	InodeNumber  uint64
	Nlinks       uint64
	ModeFlags    os.FileMode
	Uid          uint32
	Gid          uint32
	Rdev         uint64
	Size         int64
	BlockSize    int64
	NumBlocks    int64
	CreatedAt    time.Time
	LastChanged  time.Time
	LastAccessed time.Time
	LastModified time.Time
	DeletedAt    time.Time
}

func (stat *FileStat) IsDir() bool {
	return stat.ModeFlags&S_IFMT == S_IFDIR
}

func (stat *FileStat) IsFile() bool {
	return stat.ModeFlags&S_IFMT == S_IFREG
}

func (stat *FileStat) IsSymlink() bool {
	return stat.ModeFlags&S_IFMT == S_IFLNK
}

// FSStat is a platform-independent form of syscall.Statfs_t.
type FSStat struct {
	// BlockSize is the size of a logical block on the file system, in bytes.
	BlockSize int64
	// TotalBlocks is the total number of blocks on the disk image.
	TotalBlocks uint64
	// BlocksFree is the number of unallocated blocks on the image.
	BlocksFree uint64
	// BlocksAvailable is the number of blocks available for use by user data.
	// This should always be less than or equal to BlocksFree.
	BlocksAvailable uint64
	// Files is the total number of used directory entries on the file system.
	// Drivers should set this to 0 if the information is not available.
	Files uint64
	// FilesFree is the number of remaining directory entries available for use.
	// Drivers should set this to 0 for file systems that have no limit on the
	// maximum number of directory entries.
	FilesFree uint64
	// FileSystemID is the serial number for the disk image, if available.
	FileSystemID uint64
	// MaxNameLength is the longest possible name for a directory entry, in bytes.
	// Drivers should set this to 0 if there is no limit.
	MaxNameLength int64
	// Flags is free for drivers to use as they see fit, but mostly to preserve
	// flags present in the boot block. Driver-agnostic functions will ignore it.
	Flags int64
	// Label is the volume label, if available.
	Label string
}

type Truncator interface {
	Truncate(size int64) error
}

// File is the expected interface for file handles from drivers.
//
// This interface is intended to be more or less a drop-in replacement for
// `os.File`, *however* not all functions need be implemented. In particular,
// the deadline-related functions should be no-ops, and `SyscallConn()` should
// return an ENOSYS error.
type File interface {
	io.ReadWriteCloser
	io.Seeker
	io.ReaderAt
	io.ReaderFrom
	io.WriterAt
	io.StringWriter
	Truncator

	Chdir() error
	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
	Fd() uintptr
	Name() string
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)

	// SetDeadline is only present for compatibility with os.File(). Drivers
	// should implement it as a no-op.
	SetDeadline(t time.Time) error

	// SetReadDeadline is only present for compatibility with os.File(). Drivers
	// should implement it as a no-op.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline is only present for compatibility with os.File(). Drivers
	// should implement it as a no-op.
	SetWriteDeadline(t time.Time) error

	// SyscallConn is only present for compatibility with os.File(). Drivers
	// should ignore it and return an ENOSYS error immediately.
	SyscallConn() (syscall.RawConn, error)

	Stat() (os.FileInfo, error)
	Sync()
}

// Driver is the bare minimum interface for all drivers.
type Driver interface {
	// Mount initializes the driver with a file for the disk image. This must be
	// called before using the driver.
	//
	// In the event that an image is already mounted when this is called, one of
	// two things will happen: if `flags` is NOT identical to `CurrentMountFlags()`,
	// the function fails immediately with EBUSY. Otherwise, it "succeeds" and
	// returns nil. In both cases, the function should not modify the driver's
	// state.
	Mount(flags MountFlags) error

	// CurrentMountFlags returns the flags that the volume is currently mounted
	// with. If 0, the image is not mounted. This can never fail.
	CurrentMountFlags() MountFlags

	// Unmount flushes all pending changes to the disk image. The driver should
	// not be used after this function is called, except to call `Mount()` again.
	// This must fail with EBUSY if any resources are still in use, such as open
	// files.
	Unmount() error

	// GetFSInfo returns basic information about the file system. It must not be
	// called if the file system is unmounted.
	GetFSInfo() (FSStat, error)
}

// FormattingDriver is the interface for drivers capable of creating new disk
// images.
type FormattingDriver interface {
	Format(information FSStat) error
}

// OpeningDriver is the interface for drivers implementing the POSIX open(3) function.
//
// Drivers need not implement all functionality for valid flags. For example,
// read-only drivers must return an error if called with the disko.O_CREATE flag.
type OpeningDriver interface {
	// OpenFile is equivalent to os.OpenFile.
	OpenFile(path string, flag int, perm os.FileMode) (File, error)
}

// ReadingDriver is the interface for drivers supporting read operations.
type ReadingDriver interface {
	SameFile(fi1, fi2 os.FileInfo) bool
	Open(path string) (File, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use
	// a reasonable default value. For most of these 0 is fine, but for
	// compatibility drivers should use 1 for `Nlinks` and 0o777 for `ModeFlags`.
	Stat(path string) (FileStat, error)
}

type DirReadingDriver interface {
	ReadDir(path string) ([]DirectoryEntry, error)
}

// ReadingLinkingDriver provides a read-only interface for linking features on
// file systems that support links.
type ReadingLinkingDriver interface {
	Readlink(path string) (string, error)

	// Lstat returns the same information as Stat but follows symbolic links.
	Lstat(path string) (FileStat, error)
}

// WritingDriver is the interface for drivers supporting write operations.
type WritingDriver interface {
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Remove(path string) error
	Repath(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
	// Flush writes all changes to the disk image.
	Flush() error
}

type DirWritingDriver interface {
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
}

// WritingLinkingDriver provides a writing interface to linking features on file
// systems that support links.
type WritingLinkingDriver interface {
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
}

////////////////////////////////////////////////////////////////////////////////
// Directory Entries

// DirectoryEntry represents a file, directory, device, or other entity
// encountered on the file system. It must implement the os.FileInfo interface
// but only needs to fill values in Stat for the features it supports.
//
// For recommendations for how to fill the fields in Stat, see ReadingDriver.Stat().
type DirectoryEntry interface {
	os.FileInfo
	Stat() FileStat
}

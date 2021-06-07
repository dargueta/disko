package disko

import (
	"os"
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
	MountFlagsCustomStart     = MountFlags(1 << iota)
)

const MountFlagsAllowAll = MountFlagsCustomStart - 1
const MountFlagsAllowReadWrite = MountFlagsAllowRead | MountFlagsAllowWrite
const MountFlagsMask = MountFlagsAllowAll

// FileStat is a platform-independent form of syscall.Stat_t.
type FileStat struct {
	DeviceID     uint64
	InodeNumber  uint64
	Nlinks       uint64
	ModeFlags    uint32
	Uid          uint32
	Gid          uint32
	Rdev         uint64
	Size         int64
	BlockSize    int64
	Blocks       int64
	CreatedAt    time.Time
	LastChanged  time.Time
	LastAccessed time.Time
	LastModified time.Time
	DeletedAt    time.Time
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
	// maximum directory entries.
	FilesFree uint64
	// FileSystemID is the serial number for the disk image, if available.
	FileSystemID uint64
	// MaxNameLength is the longest possible name for a directory entry, in bytes.
	// Drivers should set this to 0 if there is no limit.
	MaxNameLength int64
	// Flags is
	Flags int64
	// Label is the volume label, if available.
	Label string
}

// Driver is the bare minimum interface for all drivers.
type Driver interface {
	// Mount initializes the driver with a file for the disk image. This must be
	// called before using the driver. Drivers should ignore subsequent calls to
	// Mount() before an Unmount().
	Mount(flags MountFlags) error

	// Unmount flushes all pending changes to the disk image. The driver must
	// not be used after this function is called. This must fail with EBUSY if
	// any resources are still in use, such as open files.
	Unmount() error

	// GetFSInfo returns basic information about the file system. It must not be
	// called before the file system is mounted.
	GetFSInfo() (FSStat, error)
}

// FormattingDriver is the interface for drivers capable of creating new disk
// images.
type FormattingDriver interface {
	Format(information FSStat) error
}

// OpeningDriver is the interface for drivers implementing the POSIX open(3) function.
//
// Drivers need not implement all functionality for valid flags. For example, read-only
// drivers must return an error if called with the os.O_CREATE flag.
type OpeningDriver interface {
	// OpenFile is equivalent to os.OpenFile.
	OpenFile(path string, flag int, perm os.FileMode) (*os.File, error)
}

// ReadingDriver is the interface for drivers supporting read operations.
type ReadingDriver interface {
	SameFile(fi1, fi2 os.FileInfo) bool
	Open(path string) (*os.File, error)
	ReadDir(path string) ([]DirectoryEntry, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use a
	// reasonable default value. For most of these 0 is fine, but for compatibility
	// drivers should use 1 for `Nlinks` and 0o777 for `ModeFlags`.
	Stat(path string) (FileStat, error)
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
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Repath(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (*os.File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
	// Flush writes all changes to the disk image.
	Flush() error
}

// WritingLinkingDriver provides a writing interface to linking features on file
// systems that support links.
type WritingLinkingDriver interface {
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
}

// DirectoryEntry represents a file, directory, device, or other entity
// encountered on the file system. It must implement the os.FileInfo interface
// but only needs to fill values in Stat for the features it supports. (As far
// as the os.FileInfo interface goes, drivers only need to implement Name(); all
// others have default implementations.)
//
// For recommendations for how to fill the fields in Stat, see ReadingDriver.Stat().
type DirectoryEntry struct {
	os.FileInfo
	Stat FileStat
}

// ModTime returns the timestamp of the DirectoryEntry.
func (d *DirectoryEntry) ModTime() time.Time {
	return d.Stat.LastModified
}

// Mode returns the file system mode of the directory as an os.FileMode. If you
// need more detailed information, see DirectoryEntry.Stat.
func (d *DirectoryEntry) Mode() os.FileMode {
	return os.FileMode(d.Stat.ModeFlags & 0x1ff)
}

// IsDir returns true if it's a directory.
func (d *DirectoryEntry) IsDir() bool {
	return (d.Stat.ModeFlags & S_IFDIR) != 0
}

// Sys returns a copy of the FileStat object backing this directory entry.
func (d *DirectoryEntry) Sys() FileStat {
	return d.Stat
}

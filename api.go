package disko

import (
	"os"
	"time"
)

// FileStat is a platform-independent form of syscall.Stat_t.
type FileStat struct {
	Dev          uint64
	Ino          uint64
	Nlink        uint64
	Mode         uint32
	Uid          uint32
	Gid          uint32
	Rdev         uint64
	Size         int64
	Blksize      int64
	Blocks       int64
	CreatedAt    time.Time
	LastChanged  time.Time
	LastAccessed time.Time
	LastModified time.Time
	DeletedAt    time.Time
}

// FSStat is a platform-independent form of syscall.Statfs_t.
type FSStat struct {
	BlockSize       int64
	TotalBlocks     uint64
	BlocksFree      uint64
	BlocksAvailable uint64
	Files           uint64
	FilesFree       uint64
	FileSystemID    uint64
	MaxNameLength   int64
	Flags           int64
	Label           string
}

// Driver is the bare minimum interface for all drivers.
type Driver interface {
	// Mount initializes the driver with a file for the disk image. This must be
	// called before using the driver. Drivers should ignore subsequent calls to
	// Mount() before an Unmount().
	//
	// `flags` is any combination of os.O_* flags. Drivers should ignore anything
	// they don't recognize, but reject flags they explicitly don't support. For
	// example, a driver that doesn't support creating new empty images should
	// fail if os.O_CREATE is passed in.
	Mount(flags int) error

	// Unmount flushes all pending changes to the disk image. The driver must
	// not be used after this function is called. This must fail with EBUSY if
	// any resources are still in use, such as open files.
	Unmount() error

	// GetFSInfo returns basic information about the file system. It must not be
	// called before the file system is mounted.
	GetFSInfo() (FSStat, error)
}

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
	Readlink(path string) (string, error)
	SameFile(fi1, fi2 os.FileInfo) bool
	Open(path string) (*os.File, error)
	ReadDir(path string) ([]os.FileInfo, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use a
	// reasonable default value. For most of these 0 is fine, but for compatibility
	// drivers should use 1 for `Nlink` and 0o777 for `Mode`.
	Stat(path string) (FileStat, error)
	// Lstat returns the same information as Stat but follows symbolic links. On file
	// systems that don't support symbolic links, the behavior is exactly the same as
	// Stat.
	Lstat(path string) (FileStat, error)
}

// WritingDriver is the interface for drivers supporting write operations.
type WritingDriver interface {
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Repath(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (*os.File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
	// Flush writes all changes to the disk image.
	Flush() error
}

// DirectoryEntry represents a file, directory, device, or other entity encountered on
// the file system. It must implement the os.FileInfo interface but only needs to fill
// values in Stat for the features it supports. (As far as the os.FileInfo interface goes,
// drivers only need to implement Name(); all others have default implementations.)
//
// For recommendations for how to fill the fields in Stat, see Driver.Stat().
type DirectoryEntry struct {
	os.FileInfo
	name string
	Stat FileStat
}

// Name returns the base name of the directory entry on the file system.
func (d *DirectoryEntry) Name() string {
	return d.name
}

// ModTime returns the timestamp of the DirectoryEntry.
func (d *DirectoryEntry) ModTime() time.Time {
	return d.Stat.LastModified
}

// Mode returns the file system mode of the directory as an os.FileMode. If you need more
// detailed information, see DirectoryEntry.Stat.
func (d *DirectoryEntry) Mode() os.FileMode {
	return os.FileMode(d.Stat.Mode & 0x1ff)
}

// IsDir returns true if it's a directory.
func (d *DirectoryEntry) IsDir() bool {
	return (d.Stat.Mode & S_IFDIR) != 0
}

// Sys returns a copy of the FileStat object backing this directory entry.
func (d *DirectoryEntry) Sys() FileStat {
	return d.Stat
}

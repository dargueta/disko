package disko

import (
	"os"
	"syscall"
	"time"
)

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
	Stat(path string) (syscall.Stat_t, error)
	// Lstat returns the same information as Stat but follows symbolic links. On file
	// systems that don't support symbolic links, the behavior is exactly the same as
	// Stat.
	Lstat(path string) (syscall.Stat_t, error)
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
}

// Driver is the interface for drivers implementing all driver capabilities.
type Driver interface {
	OpeningDriver
	ReadingDriver
	WritingDriver

	// Mount initializes the driver with a file for the disk image. This must not be
	// called more than once, and it must be called before using the driver. Drivers must
	// return an error in these cases, but the state of the driver after a second call
	// is undefined.
	Mount(file interface{}) error

	// Flush writes all changes to the disk image. Read-only drivers must ignore this
	// and should not return an error.
	Flush() error

	// Unmount flushes all changes to the disk image and frees all resources. The driver
	// must not be used after this function is called.
	Unmount() error
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
	Stat syscall.Stat_t
}

// Name returns the base name of the directory entry on the file system.
func (d *DirectoryEntry) Name() string {
	return d.name
}

// ModTime returns the timestamp of the DirectoryEntry.
func (d *DirectoryEntry) ModTime() time.Time {
	return time.Unix(d.Stat.Mtim.Sec, d.Stat.Mtim.Nsec)
}

// Mode returns the file system mode of the directory as an os.FileMode. If you need more
// detailed information, see DirectoryEntry.Stat.
func (d *DirectoryEntry) Mode() os.FileMode {
	return os.FileMode(d.Stat.Mode & 0x1ff)
}

// IsDir returns true if it's a directory.
func (d *DirectoryEntry) IsDir() bool {
	return (d.Stat.Mode & syscall.S_IFDIR) != 0
}

// Sys returns a copy of the syscall.Stat_t object backing this directory entry.
func (d *DirectoryEntry) Sys() interface{} {
	return d.Stat
}

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
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
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
}

// DirectoryEntry represents a file, directory, device, or other entity encountered on
// the file system. It must implement the FileInfo interface and contain all the same
// fields as syscall.Stat_t, though not all fields need to be implemented.
//
// If a file system doesn't support a particular feature, it should use a reasonable
// default value. For most of these 0 is fine, but for compatibility drivers should use
// 1 for `Nlink` and either 0o666 or 0o777 for `Mode`.
//
// BUG(dargueta): We have a collision -- os.FileInfo and syscall.Stat_t both have a field
// called `Mode`. FileInfo.Mode is a function returning an os.FileMode (uint32), and
// Stat_t.Mode is the uint32 representing the file mode flags.
type DirectoryEntry struct {
	os.FileInfo
	syscall.Stat_t
}

func (d *DirectoryEntry) ModTime() time.Time {
	return time.Unix(d.Mtim.Sec, d.Mtim.Nsec)
}

func (d *DirectoryEntry) Mode() os.FileMode {
	return os.FileMode(d.Stat_t.Mode & 0x1ff)
}

func (d *DirectoryEntry) IsDir() bool {
	return (d.Stat_t.Mode & syscall.S_IFDIR) != 0
}

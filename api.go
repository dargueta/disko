package disko

import (
	"io"
	"math"
	"os"
	"time"

	"github.com/dargueta/disko/drivers/common"
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

// FileStat is a platform-independent form of [syscall.Stat_t].
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
	return stat.ModeFlags.IsDir()
}

func (stat *FileStat) IsFile() bool {
	return stat.ModeFlags.IsRegular()
}

func (stat *FileStat) IsSymlink() bool {
	return stat.ModeFlags&os.ModeType == os.ModeSymlink
}

// FSStat is a platform-independent form of [syscall.Statfs_t].
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
	// Drivers should set this to [math.MaxUint64] for file systems that have
	// no limit on the maximum number of directory entries.
	FilesFree uint64
	// FileSystemID is the serial number for the disk image, if available.
	FileSystemID uint64
	// MaxNameLength is the longest possible name for a directory entry, in bytes.
	// Drivers should set this to [math.MaxInt64] if there is no limit.
	MaxNameLength int64
	// Flags is free for drivers to use as they see fit, but mostly to preserve
	// flags present in the boot block. Driver-agnostic functions will ignore it.
	Flags int64
	// Label is the volume label, if available.
	Label string
}

// UndefinedTimestamp is a timestamp that should be used as an invalid value,
// like `nil` for pointers.
var UndefinedTimestamp = time.UnixMicro(math.MaxInt64)

// FSFeatures indicates the features available for the file system. If a file
// system supports a feature, driver implementations MUST declare it as available
// even if the driver hasn't implemented it yet.
type FSFeatures interface {
	HasDirectories() bool
	HasSymbolicLinks() bool
	HasHardLinks() bool
	HasCreatedTime() bool
	HasAccessedTime() bool
	HasModifiedTime() bool
	HasChangedTime() bool
	HasDeletedTime() bool
	HasUnixPermissions() bool
	HasUserID() bool
	HasGroupID() bool
	HasUserPermissions() bool
	HasGroupPermissions() bool

	// TimestampEpoch returns the earliest representable timestamp on this file
	// system. File systems that don't support timestamps of any kind should
	// return [UndefinedTimestamp].
	TimestampEpoch() time.Time

	// DefaultNameEncoding gives the name of the text encoding natively used by
	// the file system, in lowercase with no symbols (e.g. "utf8" not "UTF-8").
	// For systems this old for the most part it will be either "ascii" or
	// "ebcdic".
	DefaultNameEncoding() string
	SupportsBootCode() bool

	// MaxBootCodeSize returns the maximum number of bytes that can be stored as
	// boot code in the file system. File systems that don't support boot code
	// must return 0. File systems that don't have a theoretical upper limit
	// should return [math.MaxInt].
	MaxBootCodeSize() int

	// BlockSize gives the default size of a single block in the file system,
	// in bytes. File systems that don't have fixed block sizes (such as certain
	// types of archives) should return 0.
	DefaultBlockSize() int
}

// Truncator is an interface for objects that support a Truncate() method. This
// method must behave just like [os.File.Truncate].
type Truncator interface {
	Truncate(size int64) error
}

// DriverImplementation is an interface that drivers must implement to get all
// default functionality from the CommonDriver.
type DriverImplementation interface {
	// CreateObject creates an object on the file system that is *not* a
	// directory. The following guarantees apply: A) this will never be called
	// for an object that already exists; B) `parent` will always be a valid
	// object handle.
	CreateObject(
		name string,
		parent ObjectHandle,
		perm os.FileMode,
	) (ObjectHandle, DriverError)

	// GetObject returns a handle to an object with the given name in a directory
	// specified by `parent`. The following guarantees apply: A) this will never
	// be called for a nonexistent object; B) `parent` will always be a valid
	// object handle.
	GetObject(
		name string,
		parent ObjectHandle,
	) (ObjectHandle, DriverError)

	// GetRootDirectory returns a handle to the root directory of the disk image.
	// This must always be a valid object handle, even if directories are not
	// supported by the file system (e.g. FAT8).
	GetRootDirectory() ObjectHandle

	// FSStat returns information about the file system. Multiple calls to this
	// function should return identical data if no modifications have been made
	// to the file system.
	FSStat() FSStat

	// GetFSFeatures returns an interface that gives the various features the
	// file system supports, regardless of whether the driver implements these
	// features or not.
	GetFSFeatures() FSFeatures

	FormatImage(
		image io.ReadWriteSeeker,
		stat FSStat,
	) DriverError

	// SetBootCode sets the machine code that is executed on startup if the disk
	// image is used as a boot volume. This function will never be called if the
	// [FSFeatures.SupportsBootCode] returns false.

	// If the file system doesn't have explicit
	// support for this defined in the standard (such as FAT8), it should do
	// nothing and immediately return an error with ENOSYS as the error code.
	//
	//
	SetBootCode(code []byte) DriverError
	GetBootCode() ([]byte, DriverError)
}

// ObjectHandle is an interface for a way to interact with on-disk file system
// objects.
type ObjectHandle interface {
	// Stat returns information on the status of the file as it appears on disk.
	Stat() FileStat

	// Resize changes the size of the object, in bytes. Drivers are responsible
	// for ensuring the needed number of blocks are allocated or freed.
	Resize(newSize uint64) DriverError

	// ReadBlocks fills `buffer` with data from a sequence of logical blocks
	// beginning at `index`. The following guarantees apply:
	//
	//   - `buffer` is a nonzero multiple of the size of a block.
	//   - The read range is guaranteed to be within the current boundaries of
	//     the object.
	ReadBlocks(index common.LogicalBlock, buffer []byte) DriverError

	// WriteBlocks writes bytes from `buffer` into a sequence of logical blocks
	// beginning at `index`. The following guarantees apply:
	//
	//   - `buffer` is a nonzero multiple of the size of a block.
	//   - The write range is guaranteed to be within the current boundaries of
	//     the object.
	WriteBlocks(index common.LogicalBlock, data []byte) DriverError

	// ZeroOutBlocks tells the driver to treat `count` blocks beginning at
	// `startIndex` as consisting entirely of null bytes (0). It does not change
	// the size of the object.
	//
	// Functionally, it's equivalent to:
	//
	//		buffer := make([]byte, BlockSize * NUM_BLOCKS)
	//		WriteBlocks(START_BLOCK, buffer)
	//
	// However, some file systems have optimizations for such "holes" that can
	// save disk space. If the file system doesn't support hole optimization,
	// the driver *must* set all bytes in these blocks to 0.
	//
	// NOTE: It's the driver's responsibility to consolidate holes where possible.
	ZeroOutBlocks(startIndex common.LogicalBlock, count uint) DriverError

	// Unlink deletes the file system object. For directories, this is guaranteed
	// to not be called unless [ListDir] returns an empty slice (ignoring "." and
	// ".." if present).
	Unlink() DriverError

	// Chmod changes the permission bits of this file system object. Only the
	// permissions bits will be set.
	Chmod(mode os.FileMode) DriverError
	Chown(uid, gid int) DriverError
	Chtimes(createdAt, lastAccessed, lastModified, lastChanged, deletedAt time.Time) error

	// ListDir returns a list of the directory entries this object contains. "."
	// and ".." are ignored if present.
	ListDir() ([]string, DriverError)

	// Name returns the name of the object itself without any path component.
	// The root directory, which technically has no name, must return "/".
	Name() string
}

// File is the expected interface for file handles from drivers.
//
// This interface is intended to be more or less a drop-in replacement for
// [os.File], *however* not all functions need be implemented.
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
	Name() string
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Stat() (os.FileInfo, error)
	Sync() error
}

////////////////////////////////////////////////////////////////////////////////
// Directory Entries

// DirectoryEntry represents a file, directory, device, or other entity
// encountered on the file system. It must implement the os.FileInfo interface
// but only needs to fill values in Stat for the features it supports.
//
// For recommendations for how to fill the fields in Stat, see ReadingDriver.Stat().
type DirectoryEntry interface {
	os.DirEntry
	Stat() FileStat
}

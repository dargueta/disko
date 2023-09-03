package disko

import (
	"io"
	"os"
	"time"

	"github.com/dargueta/disko/disks"
	"github.com/dargueta/disko/file_systems/common"
)

const KiB = 1024
const MiB = KiB * 1024
const GiB = MiB * 1024

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

// FileSystemImplementer is the minimum interface required for all file system
// implementations.
type FileSystemImplementer interface {
	// Mount initializes the file system implementation with access settings.
	Mount(flags MountFlags) DriverError

	// Unmount writes out all pending changes to the disk image and releases any
	// resources the implementation may be holding.
	//
	// The following guarantees apply:
	//
	// 	- This will only be called if [Mount] returned successfully.
	//	- There will be no open files or other handles (directory traversers, etc.)
	//	  and all changes will have been flushed.
	Unmount() DriverError

	// CreateObject creates an object on the file system, such as a file or
	// directory. You can tell the what it is based on the [os.FileMode] flags.
	//
	// The following guarantees apply:
	//
	// 	- This will never be called for an object that already exists.
	//  - `parent` will always be a valid object handle.
	CreateObject(
		name string,
		parent ObjectHandle,
		perm os.FileMode,
	) (ObjectHandle, DriverError)

	// GetObject returns a handle to an object with the given name in a directory
	// specified by `parent`.
	//
	// The following guarantees apply:
	//
	// 	- This will never be called for a nonexistent object.
	//	- `parent` will always be a valid object handle.
	GetObject(
		name string,
		parent ObjectHandle,
	) (ObjectHandle, DriverError)

	// GetRootDirectory returns a handle to the root directory of the disk image.
	// This must always be a valid object handle, even if directories are not
	// supported by the file system (e.g. FAT8).
	GetRootDirectory() ObjectHandle

	// FSStat returns information about the file system. Multiple calls to this
	// function must return identical data if no modifications have been made
	// to the file system.
	FSStat() FSStat

	// GetFSFeatures returns a struct that gives the various features the file
	// system supports, regardless of whether the driver implements these
	// features or not.
	//
	// This function should be callable regardless of whether the file system
	// has been mounted or not.
	GetFSFeatures() FSFeatures
}

// A FormatImageImplementer initializes an empty disk image.
type FormatImageImplementer interface {
	// FormatImage creates a new blank file system on the image this was created
	// with, using characteristics defined in `options`.
	//
	// The following guarantees apply:
	//
	//  - The image will already be correctly sized according to `options`.
	//  - It will consist entirely of null bytes.
	FormatImage(options disks.BasicFormatterOptions) DriverError
}

// A HardLinkImplementer implements hard linking from one object to another.
type HardLinkImplementer interface {
	// CreateHardLink creates a new file system object that is a hard link from
	// the source to the target.
	//
	// The following guarantees apply:
	//
	//	- The target will not already exist.
	//	- `source` will never be a directory.
	//	- `targetParentDir` will always be an existing directory.
	//
	// A thoroug explanation for why hard links are disallowed for directories
	// can be found here: https://askubuntu.com/a/525129
	CreateHardLink(
		source ObjectHandle,
		targetParentDir ObjectHandle,
		targetName string,
	) (ObjectHandle, DriverError)
}

// A BootCodeImplementer implements access to the boot code stored on a file
// system.
//
// This specifically refers to boot code defined in the file system specification,
// not a ramdisk image or something stored as a file on the file system that is
// used by the operating system for booting.
type BootCodeImplementer interface {
	// SetBootCode sets the machine code that is executed on startup if the disk
	// image is used as a boot volume. If the code provided is too short then
	// the implementation should pad it with bytes to fit. This is guaranteed to
	// be at most [FSFeatures.MaxBootCodeSize] bytes.
	SetBootCode(code []byte) DriverError

	// GetBootCode returns the machine code that is executed on startup.
	GetBootCode(buffer []byte) (int, DriverError)
}

// A VolumeLabelImplementer allows getting and setting the volume label on the
// file system for those file systems that support it.
type VolumeLabelImplementer interface {
	// SetVolumeLabel sets the volume label on the file system.
	//
	// `label` is guaranteed to be UTF-8 encoded. Implementations should convert
	// the string to the native character set if needed.
	//
	// To avoid nasty surprises, callers should try to stick to printable ASCII
	// characters and avoid non-punctuation symbols. This is because pre-1977
	// versions of the ASCII standard allowed some variance in how the same
	// character code could be depicted, e.g. `!` could also render as `|`.
	// (This is why on old keyboards the pipe character looks like `╎` to avoid
	// confusion.)
	// Thus, if you set the volume label to "(^_^)" you may get "(¬_¬)" when you
	// read it back -- a very different sentiment.
	SetVolumeLabel(label string) DriverError

	// GetVolumeLabel gets the volume label from the file system.
	//
	// The return value is guaranteed to be UTF-8 encoded. Implementations must
	// remove any padding from the return value, if applicable (and possible).
	GetVolumeLabel() (string, DriverError)
}

type ImplementerConstructor func(stream io.ReadWriteSeeker) (FileSystemImplementer, DriverError)

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
	//   - The read range will always be within the current boundaries of the
	//     object.
	ReadBlocks(index common.LogicalBlock, buffer []byte) DriverError

	// WriteBlocks writes bytes from `buffer` into a sequence of logical blocks
	// beginning at `index`. The following guarantees apply:
	//
	//   - `buffer` is a nonzero multiple of the size of a block.
	//   - The write range will always be within the current boundaries of the
	//     object.
	WriteBlocks(index common.LogicalBlock, data []byte) DriverError

	// ZeroOutBlocks tells the implementation to treat `count` logical blocks
	// beginning at `startIndex` as consisting entirely of null bytes (0). It
	// does not change the size of the object.
	//
	// Functionally, it's equivalent to:
	//
	//		buffer := make([]byte, BlockSize * NUM_BLOCKS)
	//		WriteBlocks(START_BLOCK, buffer)
	//
	// The following guarantees apply:
	//
	//   - `count` is nonzero.
	//   - `[startIndex, startIndex + count)` will always be within the current
	//     boundaries of the object.
	//
	// Some file systems have optimizations for such "holes" that can save disk
	// space. It's up to the file system implementation to handle this case, as
	// well as consolidating holes where possible. The driver doesn't care what
	// the implementation does as long as a subsequent call to [ReadBlocks] on
	// this range returns all null bytes.
	ZeroOutBlocks(startIndex common.LogicalBlock, count uint) DriverError

	// Unlink deletes the file system object. For directories, this is guaranteed
	// to not be called unless [ListDir] returns an empty slice (aside from "."
	// and ".." if present, as the driver cannot assume these exist).
	Unlink() DriverError

	// Name returns the name of the object itself without any path component.
	// The root directory, which technically has no name, must return "/".
	Name() string

	// SameAs returns a boolean indicating if this object handle refers to the
	// same on-disk object as the given handle. A few rules:
	//
	//   - Attributes such as size, timestamps, number of links, etc. should be
	//     ignored. This is only comparing identity, not properties.
	//   - Symbolic links should not be dereferenced, so X and Y are not the same
	//     even if X is a symbolic link to Y.
	//   - Hard links are considered the same as the files they refer to.
	//
	// For a UNIX-like file system, this is equivalent to comparing the inumbers.
	SameAs(other ObjectHandle) bool
}

// SupportsListDirHandle is an interface for an [ObjectHandle] that represents a
// directory to implement so that its contents can be accessed.
type SupportsListDirHandle interface {
	// ListDir returns a list of the directory entries this object contains. "."
	// and ".." are ignored if present.
	ListDir() ([]string, DriverError)
}

// SupportsChtimesHandle is an interface for an [ObjectHandle] that supports
// changing its created/modified/etc. timestamps.
type SupportsChtimesHandle interface {
	// Chtimes changes various timestamps associated with an object. The driver
	// will do its best to ensure that unsupported timestamps are equal to
	// [UndefinedTimestamp], but the implementation must ignore timestamps it
	// doesn't support.
	//
	// The following guarantees apply:
	//
	//  - Timestamps known to be unsupported (i.e. the corresponding feature in
	//    [FSFeatures] is false) will always be [UndefinedTimestamp].
	//  - `deletedAt` will only be set if the object has been deleted and the
	//    flag is supported by the file system.
	//
	// If a supported timestamp is passed in as [UndefinedTimestamp],
	// implementations should not modify the existing timestamp for the object.
	Chtimes(
		createdAt,
		lastAccessed,
		lastModified,
		lastChanged,
		deletedAt time.Time,
	) DriverError
}

// SupportsChmodHandle is an interface for an [ObjectHandle] that supports
// changing its mode flags.
type SupportsChmodHandle interface {
	// Chmod changes the permission bits of this file system object. Only the
	// permissions bits will be set in `mode`.
	//
	// File systems that support access controls but not all aspects (e.g. no
	// executable bit, or no group permissions) must silently ignore flags they
	// don't recognize.
	Chmod(mode os.FileMode) DriverError
}

// SupportsChownHandle is an interface for an [ObjectHandle] that supports
// changing its owning user and group IDs.
type SupportsChownHandle interface {
	// Chown sets the ID of the owning user and group for this object. If the
	// file system doesn't support group IDs, implementations must silently
	// ignore `gid`, whatever its value.
	Chown(uid, gid int) DriverError
}

// UndefinedTimestamp is a timestamp that should be used as an invalid value,
// equivalent to `nil` for pointers.
//
// To check if a timestamp is invalid, use [time.Time.IsZero]. Direct comparison
// to this is not recommended.
var UndefinedTimestamp = time.Time{}

const FSTextEncodingUTF8 = "utf8"
const FSTextEncodingASCII = "ascii"
const FSTextEncodingBCDIC = "bcdic"
const FSTextEncodingEBCDIC = "ebcdic"

// FSFeatures indicates the features available for the file system. If a file
// system supports a feature, driver implementations MUST declare it as available
// even if it hasn't implemented it yet.
type FSFeatures struct {
	// DoesNotRequireFormatting is true if and only if a driver doesn't need to
	// format an image before use. This is mostly only used for archive formats.
	DoesNotRequireFormatting bool

	// HasDirectories indicates if the file system supports directories, but
	// makes no assertion as to whether any nesting is supported.
	HasDirectories   bool
	HasSymbolicLinks bool
	HasHardLinks     bool
	HasCreatedTime   bool
	HasAccessedTime  bool
	HasModifiedTime  bool
	HasChangedTime   bool
	HasDeletedTime   bool

	// HasUnixPermissions is true if the file system supports a permissions model
	// similar to user/group/other, read/write/execute model that Unix uses. The
	// file system does not need to support group permissions to set this.
	HasUnixPermissions bool

	HasUserPermissions  bool
	HasGroupPermissions bool
	HasUserID           bool
	HasGroupID          bool

	// TimestampEpoch returns the earliest representable timestamp on this file
	// system. File systems that don't support timestamps of any kind must
	// return [UndefinedTimestamp] here.
	TimestampEpoch time.Time

	// DefaultNameEncoding gives the name of the text encoding natively used by
	// the file system for directory and file names (not file contents!).
	//
	// This must be lowercase with no symbols (e.g. "utf8" not "UTF-8"). For
	// systems this old it will most likely be either "ascii" or "ebcdic".
	DefaultNameEncoding string
	SupportsBootCode    bool

	// MaxBootCodeSize returns the maximum number of bytes that can be stored as
	// boot code in the file system. File systems that don't support boot code
	// must return 0. File systems that don't have a theoretical upper limit
	// should return [math.MaxInt].
	MaxBootCodeSize int

	// DefaultBlockSize gives the default size of a single block in the file
	// system, in bytes. File systems that don't have fixed block sizes (such as
	// certain types of archives) must be 0.
	DefaultBlockSize int

	// MinTotalBlocks is the smallest valid size of the file system, in blocks.
	MinTotalBlocks int64

	// MaxTotalBlocks is the largest possible size of the file system, in blocks.
	MaxTotalBlocks int64

	// MaxVolumeLabelSize gives the maximum size of the volume label for the
	// file system, in bytes. If not supported, this should be 0.
	MaxVolumeLabelSize int
}

// FileStat is a platform-independent form of [syscall.Stat_t].
//
// If a file system doesn't support a particular feature, implementations should
// use a reasonable default value. For most of these 0 is fine, but for
// compatibility they should use 1 for `Nlinks`, and 0o777 for `ModeFlags`.
// Unsupported timestamps MUST be set to [UndefinedTimestamp].
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

// IsDir returns true if and only if the file descriptor is a directory.
func (stat *FileStat) IsDir() bool {
	return stat.ModeFlags.IsDir()
}

// IsFile returns true if and only if the file descriptor is a normal file; hard
// links count as normal, symbolic links do not.
func (stat *FileStat) IsFile() bool {
	return stat.ModeFlags.IsRegular()
}

// IsSymlink returns true if and only if the file descriptor is a symbolic link,
// irrespective of what type of object the symbolic link points to.
func (stat *FileStat) IsSymlink() bool {
	return stat.ModeFlags&os.ModeType == os.ModeSymlink
}

// FSStat is a platform-independent form of [syscall.Statfs_t].
type FSStat struct {
	// BlockSize is the size of a logical block on the file system, in bytes.
	BlockSize uint

	// TotalBlocks is the total number of blocks on the disk image.
	TotalBlocks uint64

	// BlocksFree is the number of unallocated blocks on the image.
	BlocksFree uint64

	// BlocksAvailable is the number of blocks available for use by user data.
	// This should always be less than or equal to [FSStat.BlocksFree].
	BlocksAvailable uint64

	// Files is the total number of used directory entries on the file system.
	// Implementations should return 0 if the information is not available.
	Files uint64

	// FilesFree is the number of remaining directory entries available for use.
	// Implementations should return [math.MaxUint64] for file systems that have
	// no limit on the maximum number of directory entries.
	FilesFree uint64

	// FileSystemID is the serial number for the disk image, if available.
	FileSystemID string

	// MaxNameLength is the longest possible name for a directory entry, in
	// bytes. Implementations should return [math.MaxUint] if there is no limit.
	MaxNameLength uint

	// Label is the volume label, if available.
	Label string
}

// Driver is the interface implemented by the base file system driver that wraps
// a file system implementation. For most functions, the functionality is the
// same as the equivalent function in the [os] package.
//
// The presence of a function doesn't imply that the file system it's wrapping
// supports or implements that feature, so be sure to check the returned errors
// if you need something to happen.
//
// For most users, [disko.driver.BasicDriver] will meet most needs.
type Driver interface {
	// NormalizePath converts a path from the user's native file system syntax
	// to an absolute normalized path using forward slashes (/) as the component
	// separator. The return value is always an absolute path.
	NormalizePath(path string) string

	// GetFSFeatures returns a struct that gives the various features the file
	// system supports, regardless of whether the driver implements these
	// features or not.
	GetFSFeatures() FSFeatures

	// -------------------------------------------------------------------------
	// Functions from [os]

	Chdir(path string) error
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Chtimes(name string, atime time.Time, mtime time.Time) error
	Create(path string) (File, error)
	Getwd() (string, error)
	Lchown(name string, uid, gid int) error
	Link(oldname, newname string) error
	Lstat(path string) (FileStat, error)
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Open(path string) (File, error)
	OpenFile(path string, flags IOFlags, perm os.FileMode) (File, error)
	ReadDir(path string) ([]DirectoryEntry, error)
	ReadFile(path string) ([]byte, error)
	Readlink(path string) (string, error)
	Remove(path string) error
	RemoveAll(path string) error
	Rename(old string, new string) error
	SameFile(fi1, fi2 os.FileInfo) bool
	Stat(path string) (FileStat, error)
	Symlink(oldname, newname string) error
	Truncate(path string) error
	Unmount() error
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// File is the expected interface for file handles from drivers.
//
// This interface is intended to be more or less a drop-in replacement for
// [os.File], *however* not all functions are implemented. In particular, all
// deadline-related functions and `Fd` are excluded. For a full list of what is
// and isn't supported, see the documentation in the README.
type File interface {
	io.ReadWriteCloser
	io.Seeker
	io.ReaderAt
	io.ReaderFrom
	io.WriterAt
	io.StringWriter
	common.Truncator

	Chdir() error
	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
	Name() string
	ReadDir(n int) ([]os.DirEntry, error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Stat() (os.FileInfo, error)
	Sync() error
}

// DirectoryEntry represents a file, directory, device, or other entity
// encountered on the file system. It must implement the [os.FileInfo] interface
// but [Stat] only needs to fill values in [FileStat] for features it supports.
type DirectoryEntry interface {
	os.DirEntry
	Stat() FileStat
}

package basedriver


// ObjectHandle is an interface for a way to interact with on-disk file system
// objects.
type ObjectHandle interface {
	// Stat returns information on the status of the file as it appears on disk.
	Stat() disko.FileStat

	// Resize changes the size of the object, in bytes. Drivers are responsible
	// for ensuring the needed number of blocks are allocated or freed.
	Resize(newSize uint64) disko.DriverError

	// ReadBlocks fills `buffer` with data from a sequence of logical blocks
	// beginning at `index`. The following guarantees apply:
	//
	//   - `buffer` is a nonzero multiple of the size of a block.
	//   - The read range is guaranteed to be within the current boundaries of
	//     the object.
	ReadBlocks(index common.LogicalBlock, buffer []byte) disko.DriverError

	// WriteBlocks writes bytes from `buffer` into a sequence of logical blocks
	// beginning at `index`. The following guarantees apply:
	//
	//   - `buffer` is a nonzero multiple of the size of a block.
	//   - The write range is guaranteed to be within the current boundaries of
	//     the object.
	WriteBlocks(index common.LogicalBlock, data []byte) disko.DriverError

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
	ZeroOutBlocks(startIndex common.LogicalBlock, count uint) disko.DriverError

	// Unlink deletes the file system object. For directories, this is guaranteed
	// to not be called unless [ListDir] returns an empty slice (ignoring "." and
	// ".." if present).
	Unlink() disko.DriverError

	// Chmod changes the permission bits of this file system object. Only the
	// permissions bits will be set.
	Chmod(mode os.FileMode) disko.DriverError
	Chown(uid, gid int) disko.DriverError
	Chtimes(createdAt, lastAccessed, lastModified, lastChanged, deletedAt time.Time) error

	// ListDir returns a list of the directory entries this object contains. "."
	// and ".." are ignored if present.
	ListDir() ([]string, disko.DriverError)

	// Name returns the name of the object itself without any path component.
	// The root directory, which technically has no name, must return "/".
	Name() string
}

type extObjectHandle interface {
	ObjectHandle
	AbsolutePath() string
}

type tExtObjectHandle struct {
	extObjectHandle
	absolutePath string
}


// wrapObjectHandle combines
func wrapObjectHandle(handle ObjectHandle, absolutePath string) extObjectHandle {
	return tExtObjectHandle{
		ObjectHandle: handle,
		absolutePath: absolutePath,
	}
}

func (xh tExtObjectHandle) AbsolutePath() string {
	return xh.absolutePath
}

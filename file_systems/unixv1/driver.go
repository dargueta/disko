package unixv1

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
	"github.com/dargueta/disko/file_systems/common/blockcache"
)

type PhysicalBlock uint16
type LogicalBlock uint16
type Inumber uint16

const FlagFileAllocated = 0o100000
const FlagIsDirectory = 0o040000
const FlagIsModified = 0o020000 // Always set
const FlagIsLargeFile = 0o010000
const FlagSetUIDOnExecution = 0o000040 // S_ISUID
const FlagIsExecutable = 0o000020      // S_IXUSR | S_IXGRP | S_IXOTH
const FlagOwnerRead = 0o000010         // S_IRUSR
const FlagOwnerWrite = 0o000004        // S_IWUSR
const FlagNonOwnerRead = 0o000002      // S_IROTH | S_IRGRP
const FlagNonOwnerWrite = 0o000001     // S_IWOTH | S_IWGRP

// DefaultDirectoryPermissions is the default value for RawInode.Flags.
//
// This value is taken directly from the Unix v1 source code, with the single
// exception that I added in `FlagIsModified`. The file system documentation
// clearly states that `FlagIsModified` is always set, so I think that may be a
// bug in their code (unless I misread it and it's set elsewhere).
const RawDefaultDirectoryPermissions = FlagFileAllocated | FlagIsDirectory |
	FlagIsModified | FlagOwnerRead | FlagOwnerWrite | FlagNonOwnerRead

const CanonicalDefaultDirectoryPermissions = disko.S_IFDIR | disko.S_IRUSR |
	disko.S_IWUSR | disko.S_IRGRP | disko.S_IROTH

var fsEpoch time.Time = time.Date(1971, 1, 1, 0, 0, 0, 0, nil)

type UnixV1Driver struct {
	blockFreeMap      bitmap.Bitmap
	inodes            []Inode
	isMounted         bool
	image             *blockcache.BlockCache
	currentMountFlags disko.MountFlags
}

func SerializeTimestamp(tstamp time.Time) uint32 {
	return uint32(tstamp.Unix())
}

func DeserializeTimestamp(tstamp uint32) time.Time {
	return time.Unix(int64(tstamp), 0)
}

////////////////////////////////////////////////////////////////////////////////
// Implementing DriverImplementer interface
// [x] Mount
// [x] Unmount
// [ ] CreateObject
// [ ] GetObject
// [ ] GetRootDirectory
// [x] FSStat
// [ ] GetFSFeatures
// [x] FormatImage
// [ ] SetBootCode
// [ ] GetBootCode

func (driver *UnixV1Driver) Mount(
	image *blockcache.BlockCache,
	flags disko.MountFlags,
) errors.DriverError {
	if driver.isMounted {
		if driver.currentMountFlags == flags {
			return nil
		}
		return errors.ErrAlreadyInProgress
	}

	driver.currentMountFlags = flags

	// Wrap the block device with a stream for ease of reading the stuff in it.
	// For our current purposes we only need the boot block.
	bootSectorSlice, err := image.GetSlice(0, 1)
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}
	rawStream := bytes.NewReader(bootSectorSlice)

	// blockBitmapSize gives the size of the bitmap showing which blocks are in
	// use, in bytes. It's always an even number.
	var blockBitmapSize uint16
	err = binary.Read(rawStream, binary.LittleEndian, &blockBitmapSize)
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}

	// blockBitmap is the actual bitmap.
	blockBitmap := make([]byte, blockBitmapSize)
	_, err = rawStream.Read(blockBitmap)
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}

	// inodeBitmapSize is the size of the bitmap for which bitmaps are currently
	// in use, in bytes. It also is always an even number.
	var inodeBitmapSize uint16
	err = binary.Read(rawStream, binary.LittleEndian, &inodeBitmapSize)
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}

	// Together, the bitmaps can't exceed 1000 bytes because there are 24 other
	// bytes of information in the superblock that we need space for. (The
	// superblock occupies 1024 bytes, i.e. two 512-byte logical sectors.)
	if (blockBitmapSize + inodeBitmapSize) > 1000 {
		message := fmt.Sprintf(
			"corruption detected: Inode and block bitmaps can't exceed 1000"+
				" bytes together, got %d",
			blockBitmapSize+inodeBitmapSize,
		)
		return errors.ErrFileSystemCorrupted.WithMessage(message)
	}

	inodeBitmap := make([]byte, inodeBitmapSize)
	_, err = rawStream.Read(inodeBitmap)
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}

	rawInodes := make([]RawInode, inodeBitmapSize*8)
	err = binary.Read(rawStream, binary.LittleEndian, rawInodes[:])
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}

	driver.inodes = make([]Inode, inodeBitmapSize*8)

	// TODO (dargueta): Parse all raw inodes into processed ones

	driver.image = image
	driver.isMounted = true
	return nil
}

func (driver *UnixV1Driver) Unmount() errors.DriverError {
	err := driver.image.Flush()
	if err != nil {
		return errors.ErrIOFailed.WrapError(err)
	}
	driver.currentMountFlags = 0
	driver.isMounted = false
	return nil
}

func (driver *UnixV1Driver) FSStat() disko.FSStat {
	freeBlocks := uint64(0)
	for i := 0; i < int(driver.image.TotalBlocks()); i++ {
		if driver.blockFreeMap.Get(i) {
			freeBlocks++
		}
	}

	totalFiles := uint64(0)
	for _, inode := range driver.inodes {
		if inode.IsAllocated() {
			totalFiles++
		}
	}

	return disko.FSStat{
		BlockSize:       512,
		TotalBlocks:     uint64(driver.image.TotalBlocks()),
		BlocksFree:      freeBlocks,
		BlocksAvailable: uint64(driver.image.TotalBlocks()) - freeBlocks,
		Files:           totalFiles,
		FilesFree:       uint64(len(driver.inodes)),
		MaxNameLength:   8,
	}
}

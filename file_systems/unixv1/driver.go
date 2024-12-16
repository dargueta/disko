package unixv1

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
	c "github.com/dargueta/disko/file_systems/common"
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

var fsEpoch time.Time = time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)

type UnixV1Driver struct {
	blockFreeMap      bitmap.Bitmap
	inodes            []Inode
	specialInodes     [40]Inode
	isMounted         bool
	image             c.DiskImage
	currentMountFlags disko.MountFlags
}

func SerializeTimestamp(tstamp time.Time) uint32 {
	return uint32(tstamp.Unix())
}

func DeserializeTimestamp(tstamp uint32) time.Time {
	return time.Unix(int64(tstamp), 0)
}

////////////////////////////////////////////////////////////////////////////////
// FileSystemImplementer
//     [x] Mount
//     [ ] Flush
//     [x] Unmount
//     [ ] CreateObject
//     [ ] GetObject
//     [ ] GetRootDirectory
//     [x] FSStat
//     [x] GetFSFeatures
// FormatImageImplementer
//     [x] FormatImage
// HardLinkImplementer
//     [ ] CreateHardLink
// BootCodeImplementer
//     [ ] SetBootCode
//     [ ] GetBootCode
// VolumeLabelImplementer (No)

func NewDriverFromStream(stream io.ReadWriteSeeker) UnixV1Driver {
	return UnixV1Driver{
		image: blockcache.WrapStreamWithInferredSize(stream, 512, false),
	}
}

func NewDriverFromStreamWithNumBlocks(stream io.ReadWriteSeeker, totalBlocks uint) UnixV1Driver {
	return UnixV1Driver{
		image: blockcache.WrapStream(stream, 512, totalBlocks, false),
	}
}

func (driver *UnixV1Driver) Mount(flags disko.MountFlags) disko.DriverError {
	if driver.isMounted {
		// The image is already mounted. If the caller is trying to mount it again
		// with the same flags it was previously mounted with, no settings would
		// need to change so we treat this as a no-op.
		if driver.currentMountFlags == flags {
			return nil
		}
		// The caller is trying to mount this image with different flags.
		return disko.ErrAlreadyInProgress
	}

	driver.currentMountFlags = flags

	// To simplify deserialization, read the superblock into a byte slice and
	// wrap it with a stream.
	superblockBytes := make([]byte, 1024)
	nRead, err := driver.image.ReadAt(superblockBytes, c.LogicalBlock(0))
	if err != nil {
		return disko.CastToDriverError(err)
	} else if nRead != 1024 {
		return disko.ErrIOFailed.WithMessage(
			fmt.Sprintf("read failed: expected 1024B, got %d", nRead))
	}

	sbReader := bytes.NewReader(superblockBytes)

	// blockBitmapSize gives the size of the bitmap showing which blocks are in
	// use, in bytes. It's always an even number.
	var blockBitmapSize uint16
	err = binary.Read(sbReader, binary.LittleEndian, &blockBitmapSize)
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}
	if blockBitmapSize == 0 {
		return disko.ErrFileSystemCorrupted.WithMessage(
			"corruption detected: block bitmap length is 0")
	} else if blockBitmapSize%2 != 0 {
		message := fmt.Sprintf(
			"corruption detected: block bitmap length must be an even number,"+
				" got %d",
			blockBitmapSize)
		return disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	blockBitmap := make([]byte, blockBitmapSize)
	_, err = sbReader.Read(blockBitmap)
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}
	driver.blockFreeMap = bitmap.Bitmap(blockBitmap)

	// inodeBitmapSize is the size of the bitmap for which bitmaps are currently
	// in use, in bytes. It also is always an even number.
	var inodeBitmapSize uint16
	err = binary.Read(sbReader, binary.LittleEndian, &inodeBitmapSize)
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}
	if inodeBitmapSize == 0 {
		return disko.ErrFileSystemCorrupted.WithMessage(
			"corruption detected: inode bitmap length is 0")
	} else if inodeBitmapSize%2 != 0 {
		message := fmt.Sprintf(
			"corruption detected: inode bitmap length must be an even number,"+
				" got %d",
			inodeBitmapSize)
		return disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	// Together, the bitmaps can't exceed 1000 bytes because there are 24 other
	// bytes of information in the superblock that we need space for. (The
	// superblock occupies 1024 bytes, i.e. two 512-byte logical sectors.)
	if (blockBitmapSize + inodeBitmapSize) > 1000 {
		message := fmt.Sprintf(
			"corruption detected: Inode and block bitmaps can't exceed 1000"+
				" bytes together, got %d",
			blockBitmapSize+inodeBitmapSize)
		return disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	inodeBitmap := make([]byte, inodeBitmapSize)
	_, err = sbReader.Read(inodeBitmap)
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}

	totalInodes := uint(inodeBitmapSize) * 8
	bytesPerInode := driver.image.BytesPerBlock() / NumInodesPerBlock
	inodeArraySize := totalInodes * bytesPerInode

	// Read every single inode into memory. The superblock takes the first two
	// sectors (0 and 1) so we start at 2.
	inodeArrayBytes := make([]byte, inodeArraySize)
	_, err = driver.image.ReadAt(inodeArrayBytes, c.LogicalBlock(2))
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}

	driver.inodes = make([]Inode, totalInodes)

	// Deserialize the inodes we read in.
	iarrayReader := bytes.NewReader(inodeArrayBytes)
	inodeBytes := make([]byte, bytesPerInode)

	for i := 0; i < int(totalInodes); i++ {
		iarrayReader.Read(inodeBytes)
		// The first inode stored on disk is 41. 1-40 are reserved for "special
		// files" according to the documentation.
		driver.inodes[i] = BytesToInode(Inumber(i+41), inodeBytes)
	}

	driver.isMounted = true
	return nil
}

func (driver *UnixV1Driver) Unmount() disko.DriverError {
	writer, ok := driver.image.(c.WritableDiskImage)
	if !ok {
		// This is not a writable disk image so we have no changes to write out.
		return nil
	}

	err := writer.Flush()
	if err != nil {
		return disko.ErrIOFailed.Wrap(err)
	}
	driver.currentMountFlags = 0
	driver.isMounted = false
	return nil
}

func (driver *UnixV1Driver) FSStat() disko.FSStat {
	// The brute-force way to get the amount of free space is to iterate through
	// the free map and count the number of unallocated blocks. We could do this
	// via math but eh.
	freeBlocks := uint64(0)
	for i := 0; i < int(driver.image.TotalBlocks()); i++ {
		if driver.blockFreeMap.Get(i) {
			freeBlocks++
		}
	}

	totalAllocatedFiles := uint64(0)
	for _, inode := range driver.inodes {
		if inode.IsAllocated() {
			totalAllocatedFiles++
		}
	}

	return disko.FSStat{
		BlockSize:       512,
		TotalBlocks:     uint64(driver.image.TotalBlocks()),
		BlocksFree:      freeBlocks,
		BlocksAvailable: freeBlocks,
		Files:           totalAllocatedFiles,
		FilesFree:       uint64(len(driver.inodes)) - totalAllocatedFiles,
		MaxNameLength:   8,
	}
}

func (driver *UnixV1Driver) GetFSFeatures() disko.FSFeatures {
	return disko.FSFeatures{
		HasCreatedTime:      true,
		HasDirectories:      true,
		HasHardLinks:        true,
		HasUnixPermissions:  true,
		HasUserPermissions:  true,
		HasUserID:           true,
		TimestampEpoch:      fsEpoch,
		DefaultNameEncoding: "ascii",
		DefaultBlockSize:    512,
		SupportsBootCode:    true,
		MaxBootCodeSize:     32768,
		MinTotalBlocks:      66,
		MaxTotalBlocks:      8000,
	}
}

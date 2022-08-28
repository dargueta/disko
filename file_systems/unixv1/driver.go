package unixv1

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
	"github.com/dargueta/disko/drivers/common"
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
	disko.FormattingDriver // Done
	disko.OpeningDriver
	disko.ReadingDriver // Done
	BlockFreeMap        bitmap.Bitmap
	Inodes              []Inode
	isMounted           bool
	rawStream           io.ReadWriteSeeker
	image               common.BlockStream
	currentMountFlags   disko.MountFlags
}

const TimestampResolution time.Duration = time.Second / 60

func NewDriverFromStream(stream io.ReadWriteSeeker) (UnixV1Driver, error) {
	totalBlocks, err := common.DetermineBlockCount(stream, 512)
	if err != nil {
		return UnixV1Driver{}, err
	}

	blockStream := common.NewBasicBlockStream(stream, totalBlocks)
	driver := UnixV1Driver{
		rawStream: stream,
		image:     blockStream,
	}
	return driver, nil
}

func SerializeTimestamp(tstamp time.Time) uint32 {
	return uint32(tstamp.Unix())
}

func DeserializeTimestamp(tstamp uint32) time.Time {
	return time.Unix(int64(tstamp), 0)
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *UnixV1Driver) Mount(flags disko.MountFlags) error {
	if driver.isMounted {
		if driver.currentMountFlags == flags {
			return nil
		}
		return disko.NewDriverError(disko.EALREADY)
	}

	driver.currentMountFlags = flags

	var blockBitmapSize uint16
	err := binary.Read(driver.rawStream, binary.LittleEndian, &blockBitmapSize)
	if err != nil {
		return err
	}

	blockBitmap := make([]byte, blockBitmapSize)
	_, err = driver.rawStream.Read(blockBitmap)
	if err != nil {
		return err
	}

	var inodeBitmapSize uint16
	err = binary.Read(driver.rawStream, binary.LittleEndian, &inodeBitmapSize)
	if err != nil {
		return err
	}

	if (blockBitmapSize + inodeBitmapSize) > 1000 {
		message := fmt.Sprintf(
			"corruption detected: Inode and block bitmaps can't exceed 1000"+
				" bytes together, got %d",
			blockBitmapSize+inodeBitmapSize,
		)
		return disko.NewDriverErrorWithMessage(disko.EUCLEAN, message)
	}

	inodeBitmap := make([]byte, inodeBitmapSize)
	_, err = driver.rawStream.Read(inodeBitmap)
	if err != nil {
		return err
	}

	rawInodes := make([]RawInode, inodeBitmapSize*8)
	err = binary.Read(driver.rawStream, binary.LittleEndian, rawInodes[:])
	if err != nil {
		return err
	}

	driver.Inodes = make([]Inode, inodeBitmapSize*8)

	driver.isMounted = true
	return nil
}

func (driver *UnixV1Driver) CurrentMountFlags() disko.MountFlags {
	return driver.currentMountFlags
}

func (driver *UnixV1Driver) Unmount() error {
	driver.currentMountFlags = 0
	return nil
}

func (driver *UnixV1Driver) GetFSInfo() (disko.FSStat, error) {
	if !driver.isMounted {
		return disko.FSStat{}, disko.NewDriverError(disko.EIO)
	}

	freeBlocks := uint64(0)
	for i := 0; i < int(driver.image.TotalBlocks); i++ {
		if driver.BlockFreeMap.Get(i) {
			freeBlocks++
		}
	}

	totalFiles := uint64(0)
	for _, inode := range driver.Inodes {
		if inode.IsAllocated {
			totalFiles++
		}
	}

	return disko.FSStat{
		BlockSize:       512,
		TotalBlocks:     uint64(driver.image.TotalBlocks),
		BlocksFree:      freeBlocks,
		BlocksAvailable: uint64(driver.image.TotalBlocks) - freeBlocks,
		Files:           totalFiles,
		FilesFree:       uint64(len(driver.Inodes)),
		MaxNameLength:   8,
	}, nil
}

package unixv1

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
)

type PhysicalBlock uint16
type LogicalBlock uint16
type INumber uint16

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

const DefaultDirectoryPermissions = FlagFileAllocated | FlagIsDirectory | FlagIsModified |
	FlagOwnerRead | FlagOwnerWrite | FlagNonOwnerRead | FlagNonOwnerWrite

var fsEpoch time.Time = time.Date(1971, 1, 1, 0, 0, 0, 0, nil)

type RawInode struct {
	Flags            uint16
	Nlinks           uint8
	UserID           uint8
	Size             uint16
	Blocks           [8]PhysicalBlock
	CreatedTime      uint32
	LastModifiedTime uint32
	Unused           uint16
}

type RawDirent struct {
	INumber INumber
	Name    [8]byte
}

type Inode struct {
	disko.FileStat
	IsAllocated bool
	blocks      []PhysicalBlock
}

type Driver struct {
	BlockFreeMap bitmap.Bitmap
	Inodes       []Inode
	isMounted    bool
	image        *os.File
}

const TimestampResolution time.Duration = time.Second / 60

func NewDriverFromFile(file *os.File) (Driver, error) {
	driver := Driver{image: file}
	// TODO

	return driver, nil
}

func SerializeTimestamp(tstamp time.Time) uint32 {
	delta := tstamp.Sub(fsEpoch)

	// Timestamps are represented on disk in sixtieths of a second, so sixty
	// temporal units on disk are 1E9 units here.
	return uint32(delta / TimestampResolution)
}

func DeserializeTimestamp(tstamp uint32) time.Time {
	return fsEpoch.Add(time.Duration(tstamp) * TimestampResolution)
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *Driver) Mount(flags disko.MountFlags) error {
	if driver.isMounted {
		return disko.NewDriverError(disko.EALREADY)
	}

	var blockBitmapSize uint16
	err := binary.Read(driver.image, binary.LittleEndian, &blockBitmapSize)
	if err != nil {
		return err
	}

	blockBitmap := make([]byte, blockBitmapSize)
	_, err = driver.image.Read(blockBitmap)
	if err != nil {
		return err
	}

	var inodeBitmapSize uint16
	err = binary.Read(driver.image, binary.LittleEndian, &inodeBitmapSize)
	if err != nil {
		return err
	}

	if (blockBitmapSize + inodeBitmapSize) > 1000 {
		message := fmt.Sprintf(
			"corruption detected: Inode and block bitmaps can't exceed 1000"+
				" bytes together, got %d",
			blockBitmapSize+inodeBitmapSize)
		return disko.NewDriverErrorWithMessage(disko.EUCLEAN, message)
	}

	inodeBitmap := make([]byte, inodeBitmapSize)
	_, err = driver.image.Read(inodeBitmap)
	if err != nil {
		return err
	}

	driver.Inodes = make([]Inode, inodeBitmapSize*8)

	driver.isMounted = true
	return nil
}

// ConvertFSFlagsToStandard takes inode flags found in the on-disk representation
// of an inode and converts them to their standardized Unix equivalents. For
// example, `FlagNonOwnerRead` is converted to `S_IRGRP | S_IROTH`. Unrecognized
// flags are ignored.
func ConvertFSFlagsToStandard(rawFlags uint16) uint32 {
	stdFlags := uint32(0)

	if rawFlags&FlagIsDirectory != 0 {
		stdFlags |= disko.S_IFDIR
	}
	if rawFlags&FlagSetUIDOnExecution != 0 {
		stdFlags |= disko.S_ISUID
	}
	if rawFlags&FlagIsExecutable != 0 {
		stdFlags |= disko.S_IXUSR | disko.S_IXGRP | disko.S_IXOTH
	}
	if rawFlags&FlagOwnerRead != 0 {
		stdFlags |= disko.S_IRUSR
	}
	if rawFlags&FlagOwnerWrite != 0 {
		stdFlags |= disko.S_IWUSR
	}
	if rawFlags&FlagNonOwnerRead != 0 {
		stdFlags |= disko.S_IRGRP | disko.S_IROTH
	}
	if rawFlags&FlagNonOwnerWrite != 0 {
		stdFlags |= disko.S_IWGRP | disko.S_IWOTH
	}

	return stdFlags
}

// ConvertStandardFlagsToFS is the inverse of ConvertFSFlagsToStandard; it takes
// Unix mode flags and converts them to their on-disk representation.
func ConvertStandardFlagsToFS(flags uint32) uint16 {
	rawFlags := uint16(0)

	if flags&disko.S_IFDIR != 0 {
		rawFlags |= FlagIsDirectory
	}
	if flags&disko.S_ISUID != 0 {
		rawFlags |= FlagSetUIDOnExecution
	}
	if flags&disko.S_IXUSR != 0 {
		rawFlags |= FlagIsExecutable
	}
	if flags&disko.S_IRUSR != 0 {
		rawFlags |= FlagOwnerRead
	}
	if flags&disko.S_IWUSR != 0 {
		rawFlags |= FlagOwnerWrite
	}
	if flags&(disko.S_IRGRP|disko.S_IROTH) != 0 {
		rawFlags |= FlagNonOwnerRead
	}
	if flags&(disko.S_IWGRP|disko.S_IWOTH) != 0 {
		rawFlags |= FlagNonOwnerWrite
	}
	return rawFlags
}

func RawInodeToInode(inumber INumber, raw RawInode) Inode {
	sizeInBlocks := (raw.Size + (-raw.Size % 512)) / 512
	return Inode{
		IsAllocated: raw.Flags&FlagFileAllocated != 0,
		blocks:      raw.Blocks[:],
		FileStat: disko.FileStat{
			InodeNumber:  uint64(inumber),
			Nlinks:       uint64(raw.Nlinks),
			ModeFlags:    ConvertFSFlagsToStandard(raw.Flags),
			Uid:          uint32(raw.UserID),
			BlockSize:    512,
			NumBlocks:    int64(sizeInBlocks),
			Size:         int64(raw.Size),
			CreatedAt:    fsEpoch.Add(time.Second * time.Duration(raw.CreatedTime)),
			LastModified: fsEpoch.Add(time.Second * time.Duration(raw.LastModifiedTime)),
		},
	}
}

func InodeToRawInode(inode Inode) (INumber, RawInode) {
	raw := RawInode{
		Flags:  ConvertStandardFlagsToFS(inode.ModeFlags),
		Nlinks: uint8(inode.Nlinks),
		UserID: uint8(inode.Uid),
		Size:   uint16(inode.Size),
	}
	copy(raw.Blocks[:], inode.blocks)
	return INumber(inode.InodeNumber), raw
}

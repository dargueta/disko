package unixv1

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	bitmap "github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
	"github.com/dargueta/disko/file_systems/common"
	"github.com/noxer/bytewriter"
)

// getBitmapSizeInBytes returns the minimum number of bytes required to store a
// block or inode bitmap containing the given number of bits.
func getBitmapSizeInBytes(bits uint) uint {
	// Round up the number of blocks and inodes to the nearest multiple of 8,
	// then divide by 8 to give the total number of bytes each bitmap requires
	// to be stored in.
	rounded := (bits + (-bits % 8)) / 8

	// Bitmaps must be an even number of bytes.
	if rounded%2 != 0 {
		return rounded + 1
	}
	return rounded
}

// isValidBlockAndInodeCount determines if the number of inodes is valid given
// a disk of size `numBlocks`. This assumes that `numBlocks` is greater than 66.
func isValidBlockAndInodeCount(numBlocks, numInodes uint) bool {
	blockBitmapSize := getBitmapSizeInBytes(numBlocks)
	inodeBitmapSize := getBitmapSizeInBytes(numInodes)

	// The total size of both must be at most 1000 bytes.
	return (blockBitmapSize + inodeBitmapSize) <= 1000
}

func (driver *UnixV1Driver) resizeEmptyImage(stat disko.FSStat) disko.DriverError {
	// stat.Files tells us the number of inodes on the disk.
	if (stat.Files == 0) || (stat.Files%NumInodesPerBlock != 0) {
		msg := fmt.Sprintf(
			"`stat.Files` must be a non-zero multiple of %d, got %d",
			NumInodesPerBlock,
			stat.Files)
		return disko.ErrInvalidArgument.WithMessage(msg)
	}

	blockBitmapSize := getBitmapSizeInBytes(uint(stat.TotalBlocks))
	inodeBitmapSize := getBitmapSizeInBytes(uint(stat.Files))

	if !isValidBlockAndInodeCount(uint(stat.TotalBlocks), uint(stat.Files)) {
		msg := fmt.Sprintf(
			"combined free block bitmap & inode bitmaps can't exceed 1000 bytes,"+
				" got %dB (%dB and %dB respectively)",
			blockBitmapSize+inodeBitmapSize,
			blockBitmapSize,
			inodeBitmapSize)
		return disko.ErrFileTooLarge.WithMessage(msg)
	}

	// The upper 32KiB is reserved for the system (boot image and all that), and
	// the first 1 KiB is reserved for the block bitmap and inode allocation
	// bitmap. Immediately following the bitmaps is the inode array. Thus, an
	// image must be at a minimum 33 KiB (66 blocks) plus however many blocks
	// required to store `stat.Files` inodes.
	minBlocks := 66 + (stat.Files / NumInodesPerBlock)
	if stat.TotalBlocks < minBlocks {
		msg := fmt.Sprintf(
			"minimum disk image size for holding %d files is %d blocks (%.1f KiB),"+
				" got %d (%.1f KiB)",
			stat.Files,
			minBlocks,
			float64(minBlocks)/2.0,
			stat.TotalBlocks,
			float64(stat.TotalBlocks)/2.0)
		return disko.ErrInvalidArgument.WithMessage(msg)
	}

	resizer, ok := driver.image.(common.BlockDeviceResizer)
	if !ok {
		return disko.ErrNotSupported.WithMessage("the image cannot be resized.")
	}

	err := resizer.Resize(uint(stat.TotalBlocks))
	return disko.CastToDriverError(err)
}

func (driver *UnixV1Driver) Format(stat disko.FSStat) disko.DriverError {
	err := driver.resizeEmptyImage(stat)
	if err != nil {
		return err
	}

	blockBitmapSize := getBitmapSizeInBytes(uint(stat.TotalBlocks))
	inodeBitmapSize := getBitmapSizeInBytes(uint(stat.Files))
	firstDataBlock := 2 + uint(stat.Files/NumInodesPerBlock)

	outputBuffer := make([]byte, stat.BlockSize*firstDataBlock)

	bufferWriter := bytewriter.New(outputBuffer)

	// Write free block bitmap size
	binary.Write(bufferWriter, binary.LittleEndian, uint16(blockBitmapSize))

	// Initialize the bitmap  The first two blocks and last 64 are always marked
	// as allocated and can't be freed. (The first two sectors are the allocation
	// bitmaps, and the last 64 are reserved for the boot image.)
	blockBitmap := bitmap.New(int(stat.TotalBlocks))
	for i := 0; i < int(stat.TotalBlocks); i++ {
		blockBitmap.Set(i, (i >= 2) && (i < (int(stat.TotalBlocks)-64)))
	}

	// Write free block bitmap
	bufferWriter.Write(blockBitmap.Data(false))

	// Write size of inode bitmap
	binary.Write(bufferWriter, binary.LittleEndian, uint16(inodeBitmapSize))

	// Write free inode bitmap. Since a 1 indicates the inode is in use, this is
	// all null bytes.
	bufferWriter.Write(bytes.Repeat([]byte{0}, int(inodeBitmapSize)))

	// Write miscellaneous disk statistics, a total of 20 bytes. Since this is a
	// new disk, all of it is zeroes. That makes the last cold boot timestamp be
	// the Unix epoch (midnight UTC 1970-01-01) but how much do we really care?
	bufferWriter.Write(bytes.Repeat([]byte{0}, 20))

	// Write inode list. The root directory's inode always goes first.
	nowTs := SerializeTimestamp(time.Now())
	rootDirectoryInode := RawInode{
		Flags:  RawDefaultDirectoryPermissions,
		Nlinks: 1,
		UserID: 0,
		Size:   16, // Two directory entries, "." and ".."
		Blocks: [8]PhysicalBlock{
			PhysicalBlock(firstDataBlock), 0, 0, 0, 0, 0, 0, 0,
		},
		CreatedTime:      nowTs,
		LastModifiedTime: nowTs,
	}
	binary.Write(bufferWriter, binary.LittleEndian, &rootDirectoryInode)

	// Subsequent inodes go here
	inode := RawInode{Flags: FlagIsModified}
	for i := uint64(1); i < stat.Files; i++ {
		binary.Write(bufferWriter, binary.LittleEndian, &inode)
	}

	// The ilist has been completely written out. Because the number of files is
	// a multiple of NumInodesPerBlock, we're guaranteed to be at the beginning
	// of another block. This is the first data block, which we're using for the
	// root directory.
	//
	// n.b. these are directory entries for the root directory, so "." and ".."
	// are supposed to have the same values. That is not a copy-paste error.
	binary.Write(
		bufferWriter,
		binary.LittleEndian,
		RawDirent{Inumber: 41, Name: [8]byte{'.'}})
	binary.Write(
		bufferWriter,
		binary.LittleEndian,
		RawDirent{Inumber: 41, Name: [8]byte{'.', '.'}},
	)

	image, ok := driver.image.(common.WritableDiskImage)
	if !ok {
		return disko.ErrNotSupported.WithMessage("the image doesn't support writing.")
	}

	_, writeError := image.WriteAt(outputBuffer, 0)
	if writeError != nil {
		return disko.CastToDriverError(writeError)
	}
	return disko.CastToDriverError(image.Flush())
}

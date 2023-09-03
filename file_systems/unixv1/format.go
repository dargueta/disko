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

func (driver *UnixV1Driver) Format(stat disko.FSStat) disko.DriverError {
	// stat.Files tells us the number of inodes on the disk.
	if (stat.Files == 0) || (stat.Files%NumInodesPerBlock != 0) {
		msg := fmt.Sprintf(
			"`stat.Files` must be a non-zero multiple of %d, got %d",
			NumInodesPerBlock,
			stat.Files,
		)
		return disko.ErrInvalidArgument.WithMessage(msg)
	}

	blockBitmapSize := getBitmapSizeInBytes(uint(stat.TotalBlocks))
	inodeBitmapSize := getBitmapSizeInBytes(uint(stat.Files))
	firstDataBlock := 2 + uint(stat.Files/NumInodesPerBlock)

	if !isValidBlockAndInodeCount(uint(stat.TotalBlocks), uint(stat.Files)) {
		msg := fmt.Sprintf(
			"combined free block bitmap & inode bitmaps can't exceed 1000 bytes,"+
				" got %dB (%dB and %dB respectively)",
			blockBitmapSize+inodeBitmapSize,
			blockBitmapSize,
			inodeBitmapSize,
		)
		return disko.ErrFileTooLarge.WithMessage(msg)
	}

	// The upper 32KiB is reserved for the system (boot image and all that), and
	// the first 1 KiB is reserved for the block bitmap and inode allocation
	// bitmap. Immediately following the bitmaps is the inode array. Thus, an
	// image must be 33 KiB (66 blocks) plus however many blocks it takes to
	// store `stat.Files` inodes.
	minBlocks := 66 + (stat.Files / NumInodesPerBlock)
	if stat.TotalBlocks < minBlocks {
		msg := fmt.Sprintf(
			"minimum disk image size is %d blocks (%.1f KiB), got %d (%.1f KiB)",
			minBlocks,
			float64(minBlocks)/2.0,
			stat.TotalBlocks,
			float64(stat.TotalBlocks)/2.0,
		)
		return disko.ErrInvalidArgument.WithMessage(msg)
	}

	image := driver.image.(common.WritableDiskImage)

	err := image.(common.BlockDeviceResizer).Resize(uint(stat.TotalBlocks))
	if err != nil {
		return disko.CastToDriverError(err)
	}

	outputSlice, err := image.GetSlice(0, firstDataBlock)
	if err != nil {
		return disko.CastToDriverError(err)
	}

	writer := bytewriter.New(outputSlice)

	// Write free block bitmap size
	binary.Write(writer, binary.LittleEndian, uint16(blockBitmapSize))

	blockBitmap := bitmap.New(int(stat.TotalBlocks))
	for i := 0; i < int(stat.TotalBlocks); i++ {
		// The first two blocks and last 64 are always marked as allocated and
		// can't be freed. (The first two sectors are the allocation bitmaps,
		// and the last 64 are reserved for the boot image.)
		blockBitmap.Set(i, (i >= 2) && (i < (int(stat.TotalBlocks)-64)))
	}

	// Write free block bitmap
	writer.Write(blockBitmap.Data(false))

	// Write size of inode bitmap
	binary.Write(writer, binary.LittleEndian, uint16(inodeBitmapSize))

	// Write free inode bitmap. Since a 1 indicates the inode is in use, this is
	// all null bytes.
	writer.Write(bytes.Repeat([]byte{0}, int(inodeBitmapSize)))

	// Write miscellaneous disk statistics, a total of 20 bytes. Since this is a
	// new disk, all of it is zeroes. That makes the last cold boot timestamp be
	// the Unix epoch (midnight UTC 1970-01-01) but how much do we really care?
	writer.Write(bytes.Repeat([]byte{0}, 20))

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
	binary.Write(writer, binary.LittleEndian, &rootDirectoryInode)

	// Subsequent inodes go here
	inode := RawInode{Flags: FlagIsModified}
	for i := uint64(1); i < stat.Files; i++ {
		binary.Write(writer, binary.LittleEndian, &inode)
	}

	// The ilist has been completely written out. Because the number of files is
	// a multiple of NumInodesPerBlock, we're guaranteed to be at the beginning
	// of another block. This is the first data block, which we're using for the
	// root directory.
	//
	// n.b. these are directory entries for the root directory, so "." and ".."
	// are supposed to be the same. That is not a bug.
	binary.Write(
		writer,
		binary.LittleEndian,
		RawDirent{Inumber: 41, Name: [8]byte{'.'}})
	binary.Write(
		writer,
		binary.LittleEndian,
		RawDirent{Inumber: 41, Name: [8]byte{'.', '.'}},
	)

	image.MarkBlockRangeDirty(0, firstDataBlock)
	return disko.CastToDriverError(image.Flush())
}

/*
// determineMaxInodeCount gives the maximum number of inodes that can be stored
// on a disk given the size of the disk, in blocks.
//
// This gives an upper bound on the number of inodes that can be stored; it does
// *not* ensure that there will be any blocks left over for storing file data.
// It's highly unlikely you'll want to set the inode count this high.
func determineMaxInodeCount(totalBlocks uint) (uint, error) {
	if totalBlocks < 66 {
		return 0, fmt.Errorf("disk must be at least 66 blocks, got %d", totalBlocks)
	}

	// Determine the size of the block allocation bitmap on disk. We have one
	// bit per block, rounded up to the nearest multiple of two bytes (16).
	blockBitmapSize := getBitmapSizeInBytes(totalBlocks)

	// The block allocation and inode allocation bitmaps must be at most 1000
	// bytes together.
	if blockBitmapSize >= 1000 {
		return 0, errors.NewWithMessage(
			errors.EINVAL,
			"block allocation bitmap doesn't fit into the superblock",
		)
	}

	// Compute the maximum size of the inode bitmap by taking the remaining
	// space in the superblock, and rounding *down* to the nearest even number
	// of bytes.
	inodeBitmapSize := 1000 - blockBitmapSize
	if inodeBitmapSize%2 != 0 {
		inodeBitmapSize--
	}

	if inodeBitmapSize == 0 {
		return 0, errors.NewWithMessage(
			errors.EINVAL,
			"block allocation bitmap leaves no space for inode bitmap",
		)
	}

	// The maximum number of inodes is *not* just inodeBitmapSize times 8; we
	// need to take into account the number of blocks the ilist is going to use
	// as well.
	//
	// The number of blocks used for the ilist is computed as follows:
	//
	// 1. Inode bitmap size in bytes * 8 gives total number of inodes
	// 2. Multiply by 32 bytes per inode to get total bytes used by ilist
	// 3. Divide by 512 to get number of blocks
	//
	// That gives us: inodeBitmapSize * 8 * NumInodesPerBlock / 512 --> inodeBitmapSize / 2
	//
	// We know that the inode bitmap size is always an even number, so we don't
	// need to take a remainder into account.
	inodeBlocksUsed := inodeBitmapSize * 8 * NumInodesPerBlock / 512
	availableBlocks := totalBlocks - 66

	if inodeBlocksUsed > availableBlocks {
		// The computed size of the ilist exceeds the number of blocks left on
		// the disk. Adjust the number down.
		return availableBlocks * NumInodesPerBlock, nil
	}
	return inodeBlocksUsed * NumInodesPerBlock, nil
}
*/

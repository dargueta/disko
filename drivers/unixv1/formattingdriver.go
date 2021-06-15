package unixv1

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	bitmap "github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
)

func IsValidBlockAndInodeCount(numBlocks, numInodes uint) bool {
	blockBitmapSize := (numBlocks + (-numBlocks % 8)) / 8
	inodeBitmapSize := (numInodes + (-numInodes % 8)) / 8

	if blockBitmapSize%2 != 0 {
		blockBitmapSize++
	}
	if inodeBitmapSize%2 != 0 {
		inodeBitmapSize++
	}

	return (blockBitmapSize + inodeBitmapSize) <= 1000
}

func (driver *Driver) Format(stat disko.FSStat) error {
	if driver.isMounted {
		return disko.NewDriverError(disko.EBUSY)
	}

	// stat.Files tells us the number of inodes on the disk.
	if (stat.Files == 0) || (stat.Files%16 != 0) {
		msg := fmt.Sprintf(
			"`stat.Files` must be a non-zero multiple of 16, got %d",
			stat.Files)
		return disko.NewDriverErrorWithMessage(disko.EINVAL, msg)
	}

	// The free block bitmap contains one bit per block, rounded up to the
	// nearest even number of bytes.
	blockBitmapSize := (stat.TotalBlocks + (-stat.TotalBlocks % 8)) / 8
	if blockBitmapSize%2 != 0 {
		blockBitmapSize++
	}

	// Same deal with the inode free bitmap -- one bit per inode, rounded up to
	// the nearest even number of bytes.
	inodeBitmapSize := (stat.Files + (-stat.Files % 8)) / 8
	if inodeBitmapSize%2 != 0 {
		inodeBitmapSize++
	}

	if !IsValidBlockAndInodeCount(uint(stat.TotalBlocks), uint(stat.Files)) {
		msg := fmt.Sprintf(
			"combined free block bitmap & inode bitmaps can't exceed 1000 bytes,"+
				" got %dB (%dB and %dB respectively)",
			blockBitmapSize+inodeBitmapSize,
			blockBitmapSize,
			inodeBitmapSize,
		)
		return disko.NewDriverErrorWithMessage(disko.E2BIG, msg)
	}

	// The upper 32KiB is reserved for the system (boot image and all that), and
	// the first 1 KiB is reserved for the block bitmap and inode allocation
	// bitmap. Immediately following the bitmaps is the inode array. Thus, an
	// image must be 33 KiB (66 blocks) plus however many blocks it takes to
	// store `stat.Files` inodes.
	minBlocks := 66 + (stat.Files / 16)
	if stat.TotalBlocks < minBlocks {
		msg := fmt.Sprintf(
			"minimum disk image size is %d blocks (%.1f KiB), got %d (%.1f KiB)",
			minBlocks,
			float64(minBlocks)/2.0,
			stat.TotalBlocks,
			float64(stat.TotalBlocks)/2.0,
		)
		return disko.NewDriverErrorWithMessage(disko.EINVAL, msg)
	}

	driver.image.Truncate(int64(stat.TotalBlocks) * 512)
	driver.image.Seek(0, io.SeekStart)

	var wbuf [2]byte

	// Write free block bitmap size
	binary.LittleEndian.PutUint16(wbuf[:], uint16(blockBitmapSize))
	driver.image.Write(wbuf[:])

	blockBitmap := bitmap.New(int(stat.TotalBlocks))
	for i := 0; i < int(stat.TotalBlocks); i++ {
		// The first two blocks and last 64 are always marked as allocated and
		// can't be freed.
		blockBitmap.Set(i, (i >= 2) && (i < (int(stat.TotalBlocks)-64)))
	}

	// Write free block bitmap
	driver.image.Write(blockBitmap.Data(false))

	// Write size of inode bitmap
	binary.LittleEndian.PutUint16(wbuf[:], uint16(inodeBitmapSize))
	driver.image.Write(wbuf[:])

	// Write free inode bitmap. Since a 1 indicates the inode is in use, this is
	// all null bytes.
	driver.image.Write(bytes.Repeat([]byte{0}, int(inodeBitmapSize)))

	// Write miscellaneous disk statistics, a total of 20 bytes. Since this is a
	// new disk, all of it is zeroes.
	driver.image.Write(bytes.Repeat([]byte{0}, 20))

	firstDataBlock := 2 + (inodeBitmapSize / 2)

	// Write inode list. The root directory's inode always goes first.
	nowTs := SerializeTimestamp(time.Now())
	rootDirectoryInode := RawInode{
		Flags:            RawDefaultDirectoryPermissions,
		Nlinks:           1,
		UserID:           0,
		Size:             16, // Two directory entries, "." and ".."
		Blocks:           [8]PhysicalBlock{0, 0, 0, 0, 0, 0, 0, 0},
		CreatedTime:      nowTs,
		LastModifiedTime: nowTs,
	}
	rootDirectoryInode.Blocks[0] = PhysicalBlock(firstDataBlock)
	binary.Write(driver.image, binary.LittleEndian, &rootDirectoryInode)

	// Subsequent inodes go here
	inode := RawInode{Flags: FlagIsModified}
	for i := 1; i < int(inodeBitmapSize)*8; i++ {
		err := binary.Write(driver.image, binary.LittleEndian, &inode)
		if err != nil {
			return err
		}
	}

	// The ilist has been completely written out. Seek into the first data block
	// and write the "." and ".." entries for the root directory.
	driver.image.Seek(stat.BlockSize*int64(firstDataBlock), io.SeekStart)
	binary.Write(
		driver.image,
		binary.LittleEndian,
		RawDirent{INumber: 41, Name: [8]byte{'.'}})
	binary.Write(
		driver.image,
		binary.LittleEndian,
		RawDirent{INumber: 41, Name: [8]byte{'.', '.'}})

	return nil
}

// DetermineMaxInodeCount gives the maximum number of inodes that can be stored
// on a disk given the size of the disk, in blocks.
//
// This gives an upper bound on the number of inodes that can be stored; it does
// *not* ensure that there will be any blocks left over for storing file data.
// It's highly unlikely you'll want to set the inode count this high.
func DetermineMaxInodeCount(totalBlocks uint) (uint, error) {
	if totalBlocks < 66 {
		return 0, fmt.Errorf("disk must be at least 66 blocks, got %d", totalBlocks)
	}

	// Determine the size of the block allocation bitmap on disk. We have one
	// bit per block, rounded up to the nearest multiple of two bytes (16).
	blockBitmapSize := (totalBlocks + (-totalBlocks % 8)) / 8
	if blockBitmapSize%2 != 0 {
		blockBitmapSize++
	}

	// The block allocation and inode allocation bitmaps must be at most 1000
	// bytes together.
	if blockBitmapSize >= 1000 {
		return 0, errors.New("block allocation bitmap doesn't fit into the superblock")
	}

	// Compute the maximum size of the inode bitmap by taking the remaining
	// space in the superblock, and rounding *down* to the nearest even number
	// of bytes.
	inodeBitmapSize := 1000 - blockBitmapSize
	if inodeBitmapSize%2 != 0 {
		inodeBitmapSize--
	}

	if inodeBitmapSize == 0 {
		return 0, errors.New("block allocation bitmap leaves no space for inode bitmap")
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
	// That gives us: inodeBitmapSize * 8 * 32 / 512 --> inodeBitmapSize / 2
	//
	// We know that the inode bitmap size is always an even number, so we don't
	// need to take a remainder into account.
	inodeBlocksUsed := inodeBitmapSize / 2
	availableBlocks := totalBlocks - 66

	if inodeBlocksUsed > availableBlocks {
		// The computed size of the ilist exceeds the number of blocks left on
		// the disk. Adjust the number down.
		return availableBlocks * 16, nil
	}
	return inodeBlocksUsed * 16, nil
}

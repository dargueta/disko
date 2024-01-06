package unixv1

import (
	_ "embed"
	"encoding/binary"
	"testing"

	"github.com/dargueta/disko"
	dt "github.com/dargueta/disko/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/min-files-max-blocks.imgz
var diskImageFormatMinFilesMaxBlocks []byte

func TestReadExistingFormat16FileMaxSize(t *testing.T) {
	driver := newDriverFromCompressedBytes(t, diskImageFormatMinFilesMaxBlocks, 7984)

	// Validate the uncompressed image. The block and inode bitmaps together must
	// be 1000 bytes or less and both must also be even numbers. Having the max
	// blocks and min inodes gives us 998 bytes for the block bitmap and 2 for
	// the inode bitmap.
	bootSector := make([]byte, 1024)
	driver.image.ReadAt(bootSector, 0)
	require.Equal(
		t,
		998,
		int(binary.LittleEndian.Uint16(bootSector[:2])),
		"uncompressed image is invalid; block bitmap size is wrong")
	require.Equal(
		t,
		2,
		int(binary.LittleEndian.Uint16(bootSector[1000:1002])),
		"uncompressed image is invalid; inode bitmap size is wrong")

	// BUG: Entire first sector is getting skipped somehow
	err := driver.Mount(disko.MountFlagsAllowAll)
	require.NoError(t, err, "mounting failed")

	const totalBlocks = uint64(7984)

	stat := driver.FSStat()
	assert.EqualValues(t, 512, stat.BlockSize, "BlockSize")
	assert.EqualValues(t, totalBlocks, stat.TotalBlocks, "TotalBlocks")
	assert.EqualValues(t, 1, stat.Files, "Files")          // The root directory
	assert.EqualValues(t, 15, stat.FilesFree, "FilesFree") // 16 files minus root directory

	// Used blocks:
	// 64 required for boot code
	// 2 for superblock
	// ilist: 16 inodes @ 32 bytes per inode = 512B = 1 block
	// 1 block for root directory contents
	// = 68
	assert.EqualValues(t, totalBlocks-68, stat.BlocksAvailable, "BlocksAvailable")
	assert.EqualValues(t, totalBlocks-68, stat.BlocksFree, "BlocksFree")

	err = driver.Unmount()
	assert.NoError(t, err, "unmounting failed")
}

func newDriverFromCompressedBytes(
	t *testing.T, compressedImageBytes []byte, totalSectors uint,
) UnixV1Driver {
	imageStream := dt.LoadDiskImage(t, compressedImageBytes, 512, totalSectors)

	driver := NewDriverFromStreamWithNumBlocks(imageStream, totalSectors)
	require.Equal(t, uint(512), driver.image.BytesPerBlock())
	require.Equal(t, totalSectors, driver.image.TotalBlocks())
	assert.Equal(t, int64(totalSectors*512), driver.image.Size())
	return driver
}

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

//go:embed testdata/max-size-with-512-files.img.rle.gz
var diskImageFormat64FileMaxSize []byte

func TestReadExistingFormat64FileMaxSize(t *testing.T) {
	driver := newDriverFromCompressedBytes(t, diskImageFormat64FileMaxSize, 7984)

	// Validate the uncompressed image. The block and inode bitmaps together must
	// be 1000 bytes or less and both must also even numbers. Having the max
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

	stat := driver.FSStat()
	assert.Equal(t, 512, stat.BlockSize)
	assert.Equal(t, 7984, stat.TotalBlocks)
	assert.Equal(t, 1, stat.Files)      // The root directory
	assert.Equal(t, 63, stat.FilesFree) // 64 files minus root directory
	assert.Equal(t, 7984-68, stat.BlocksAvailable)
	assert.Equal(t, 7984-68, stat.BlocksFree)

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

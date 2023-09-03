package blockcache_test

import (
	"bytes"
	"math/rand"
	"testing"

	c "github.com/dargueta/disko/file_systems/common"
	diskotest "github.com/dargueta/disko/testing"
	"github.com/stretchr/testify/assert"
)

// Test block fetch functionality with no trickery such as reading past the end
// of the image.
func TestBlockCache__Fetch__Basic(t *testing.T) {
	// Disk image is 64 blocks, 128 bytes per block. 128 is a common block size
	// in very old *true* floppies.
	rawBlocks := diskotest.CreateRandomImage(128, 64, t)
	cache := diskotest.CreateDefaultCache(128, 64, false, rawBlocks, t)

	currentBlock := make([]byte, 128)
	for i := c.LogicalBlock(0); i < 64; i++ {
		_, err := cache.ReadAt(currentBlock, i)
		if err != nil {
			t.Errorf("failed to read block %d of [0, 64): %s", i, err.Error())
			continue
		}

		start := i * 128
		if !bytes.Equal(currentBlock, rawBlocks[start:start+128]) {
			t.Errorf("block %d read from the cache doesn't match", i)
		}
	}
}

// Trying to read past the end of an image must fail.
func TestBlockCache__Fetch__ReadPastEnd(t *testing.T) {
	cache := diskotest.CreateDefaultCache(512, 16, false, nil, t)
	buffer := make([]byte, 512)

	// Read the first block, should be okay.
	nRead, err := cache.ReadAt(buffer, 0)
	assert.NoError(t, err, "failed to read first block")
	assert.Equal(t, len(buffer), nRead)

	// Read the last valid block, should be okay.
	nRead, err = cache.ReadAt(buffer, 15)
	assert.NoError(t, err, "failed to read last block")
	assert.Equal(t, len(buffer), nRead)

	// Read one block past the last valid block (equal to the total number of
	// blocks). This must fail.
	nRead, err = cache.ReadAt(buffer, 16)
	assert.Error(t, err, "tried reading block 16 of [0, 16) but it didn't fail")
	assert.Equal(t, 0, nRead)

	// Try reading zero bytes at one block past the last valid block. This should
	// also fail.
	nRead, err = cache.ReadAt([]byte{}, 16)
	assert.Error(t, err, "tried reading 0 bytes of block 16 of [0, 16) but it didn't fail")
	assert.Equal(t, 0, nRead)

	nRead, err = cache.ReadAt(make([]byte, 8192), 0)
	assert.NoError(t, err, "failed reading entire image into buffer")
	assert.EqualValues(t, cache.Size(), nRead)

	nRead, err = cache.ReadAt(make([]byte, 8193), 0)
	assert.Error(t, err, "should've failed to read entire image + 1 byte into buffer")
	assert.Equal(t, 0, nRead)
}

// Write to a block and then read back that same block. You should always get
// back what you wrote.
func TestBlockCache__Write__Basic(t *testing.T) {
	cache := diskotest.CreateDefaultCache(512, 16, true, nil, t)
	writeBuffer := make([]byte, cache.BytesPerBlock())
	readBuffer := make([]byte, cache.BytesPerBlock())

	for i := 0; i < int(cache.TotalBlocks()); i++ {
		rand.Read(writeBuffer)
		cache.WriteAt(writeBuffer, c.LogicalBlock(i))
		cache.ReadAt(readBuffer, c.LogicalBlock(i))

		assert.Equalf(
			t, writeBuffer, readBuffer, "wrote to block %d but read back different data", i)
	}
}

// Attempting to write starting past the end of the cache fails.
func TestBlockCache__Write__WriteStartingPastEndFails(t *testing.T) {
	cache := diskotest.CreateDefaultCache(512, 16, true, nil, t)
	writeBuffer := make([]byte, cache.BytesPerBlock())

	n, err := cache.WriteAt(writeBuffer, c.LogicalBlock(16))
	assert.Error(t, err, "writing past the end of the buffer should've failed but it didn't")
	assert.Equal(t, 0, n)
}

// If we write to a block inside the cache but the buffer extends past the end
// of the cache, it fails immediately and no data is modified.
func TestBlockCache__Write__WriteOverlappingPastEndFails(t *testing.T) {
	cache := diskotest.CreateDefaultCache(512, 16, true, nil, t)
	cacheData, _ := cache.Data()
	copyOfOriginalData := make([]byte, len(cacheData))
	copy(copyOfOriginalData, cacheData)

	writeBuffer := make([]byte, cache.BytesPerBlock()*5)
	rand.Read(writeBuffer)

	n, err := cache.WriteAt(writeBuffer, c.LogicalBlock(12))
	assert.Error(t, err, "writing past the end of the buffer should've failed but it didn't")
	assert.Equal(t, 0, n)
	assert.Equal(t, copyOfOriginalData, cacheData, "cache data was modified but shouldn't've been")
}

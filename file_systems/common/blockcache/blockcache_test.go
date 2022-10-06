package blockcache_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dargueta/disko/errors"
	c "github.com/dargueta/disko/file_systems/common"
	"github.com/dargueta/disko/file_systems/common/blockcache"
)

// Create an image with the given number of blocks and bytes per block. It is
// guaranteed to either return a valid slice or fail the test and abort.
func createRandomImage(bytesPerBlock, totalBlocks uint, t *testing.T) []byte {
	backingData := make([]byte, bytesPerBlock*totalBlocks)

	_, err := rand.Read(backingData)
	if err != nil {
		t.Fatalf(
			"failed to initialize %d blocks of size %d with random bytes: %s",
			totalBlocks,
			bytesPerBlock,
			err.Error(),
		)
	}
	return backingData
}

// Create a cache with default settings, fetch/flush handlers, etc. The image
// cannot be resized.
//
// Arguments:
//
//   - bytesPerBlock: The number of bytes in a single block.
//   - totalBlocks: The number of blocks in the cache.
//   - writable: `true` if the image is writable, `false` otherwise. The handler
//     will fail a test if an attempt is made to write to the image if this is
//     false.
//   - backingData: Optional. A byte slice of at least `bytesPerBlock * totalBlocks`
//     that is used as the underlying storage the cache sits on top of. You can
//     pass `nil` for this to get completely random data.
//   - `t`: The testing fixture.
//
// The fetch and flush handlers generated for the cache check bounds and
// permissions for you, and fail with an appropriate error message. This means
// you won't be able to test negative conditions (i.e. ensure methods fail where
// they should) so you'll have to do that yourself. See [createRandomImage].
func createDefaultCache(
	bytesPerBlock,
	totalBlocks uint,
	writable bool,
	backingData []byte,
	t *testing.T,
) *blockcache.BlockCache {
	if backingData == nil {
		backingData = createRandomImage(bytesPerBlock, totalBlocks, t)
	}

	fetchCallback := func(blockIndex c.LogicalBlock, buffer []byte) error {
		if blockIndex >= c.LogicalBlock(totalBlocks) {
			message := fmt.Sprintf(
				"attempted to read outside bounds: %d not in [0, %d)",
				blockIndex,
				totalBlocks,
			)
			t.Error(message)
			return errors.NewWithMessage(errors.EIO, message)
		}

		start := blockIndex * c.LogicalBlock(bytesPerBlock)
		copy(buffer, backingData[start:start+c.LogicalBlock(bytesPerBlock)])
		return nil
	}

	var flushCallback blockcache.FlushBlockCallback
	if writable {
		flushCallback = func(blockIndex c.LogicalBlock, buffer []byte) error {
			if blockIndex >= c.LogicalBlock(totalBlocks) {
				message := fmt.Sprintf(
					"attempted to write outside bounds: %d not in [0, %d)",
					blockIndex,
					totalBlocks,
				)
				t.Errorf(message)
				return errors.NewWithMessage(errors.EIO, message)
			}

			start := blockIndex * c.LogicalBlock(bytesPerBlock)
			copy(backingData[start:start+c.LogicalBlock(bytesPerBlock)], buffer)
			return nil
		}
	} else {
		flushCallback = func(blockIndex c.LogicalBlock, buffer []byte) error {
			message := fmt.Sprintf(
				"attempted to write %d bytes to block %d of read-only image",
				len(buffer),
				blockIndex,
			)
			t.Errorf(message)
			return errors.NewWithMessage(errors.EROFS, message)
		}
	}

	cache := blockcache.New(
		bytesPerBlock, totalBlocks, fetchCallback, flushCallback, nil,
	)
	if cache.BytesPerBlock() != bytesPerBlock {
		t.Errorf(
			"wrong bytes per block: %d != %d", cache.BytesPerBlock(), bytesPerBlock,
		)
	}

	if cache.TotalBlocks() != totalBlocks {
		t.Errorf("wrong total blocks: %d != %d", cache.TotalBlocks(), totalBlocks)
	}

	return cache
}

// Test block fetch functionality with no trickery such as reading past the end
// of the image.
func TestBlockCache__Fetch__Basic(t *testing.T) {
	// Disk image is 64 blocks, 128 bytes per block. 128 is a common block size
	// in very old *true* floppies.
	rawBlocks := createRandomImage(128, 64, t)
	cache := createDefaultCache(128, 64, false, rawBlocks, t)

	currentBlock := make([]byte, 128)
	for i := c.LogicalBlock(0); i < 64; i++ {
		err := cache.Read(i, currentBlock)
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
	cache := createDefaultCache(512, 16, false, nil, t)
	buffer := make([]byte, 512)

	// Read the first block, should be okay.
	err := cache.Read(0, buffer)
	if err != nil {
		t.Errorf("failed to read first block: %s", err.Error())
	}

	// Read the last valid block, should be okay.
	err = cache.Read(15, buffer)
	if err != nil {
		t.Errorf("failed to read last block: %s", err.Error())
	}

	// Read one block past the last valid block (equal to the total number of
	// blocks). This must fail.
	err = cache.Read(16, buffer)
	if err == nil {
		t.Error("tried reading block 16 of [0, 16) but it didn't fail")
	}

	// Try reading zero bytes at one block past the last valid block. This should
	// also fail.
	err = cache.Read(16, []byte{})
	if err == nil {
		t.Error("tried reading 0 bytes of block 16 of [0, 16) but it didn't fail")
	}

	err = cache.Read(0, make([]byte, 8192))
	if err != nil {
		t.Errorf("failed reading entire image into buffer: %s", err.Error())
	}

	err = cache.Read(0, make([]byte, 8193))
	if err == nil {
		t.Error("should've failed to read entire image + 1 byte into buffer")
	}
}

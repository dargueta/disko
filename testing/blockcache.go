package testing

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/dargueta/disko/errors"
	c "github.com/dargueta/disko/file_systems/common"
	"github.com/dargueta/disko/file_systems/common/blockcache"
)

// Create an image with the given number of blocks and bytes per block. It is
// guaranteed to either return a valid slice or fail the test and abort.
func CreateRandomImage(bytesPerBlock, totalBlocks uint, t *testing.T) []byte {
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
func CreateDefaultCache(
	bytesPerBlock,
	totalBlocks uint,
	writable bool,
	backingData []byte,
	t *testing.T,
) *blockcache.BlockCache {
	if backingData == nil {
		backingData = CreateRandomImage(bytesPerBlock, totalBlocks, t)
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

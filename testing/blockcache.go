package testing

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/dargueta/disko"
	c "github.com/dargueta/disko/file_systems/common"
	"github.com/dargueta/disko/file_systems/common/blockcache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create an image with the given number of blocks and bytes per block. It is
// guaranteed to either return a valid slice or fail the test and abort.
func CreateRandomImage(bytesPerBlock, totalBlocks uint, t *testing.T) []byte {
	backingData := make([]byte, bytesPerBlock*totalBlocks)

	_, err := rand.Read(backingData)
	require.NoErrorf(
		t,
		err,
		"failed to initialize %d blocks of size %d with random bytes",
		totalBlocks,
		bytesPerBlock,
	)
	return backingData
}

// CreateDefaultCache creates a block cache with default settings, fetch/flush
// handlers, etc. The image cannot be resized.
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
// The fetch and flush handlers check bounds and permissions for you, and fail
// with an appropriate error message. This means you won't be able to test
// negative conditions (i.e. ensure methods fail where they should) so you'll
// have to do that yourself. See [CreateRandomImage].
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
				"attempted to read outside bounds: block %d not in [0, %d)",
				blockIndex,
				totalBlocks,
			)
			t.Error(message)
			return disko.ErrIOFailed.WithMessage(message)
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
				t.Error(message)
				return disko.ErrIOFailed.WithMessage(message)
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
			t.Error(message)
			return disko.ErrReadOnlyFileSystem.WithMessage(message)
		}
	}

	cache := blockcache.New(
		bytesPerBlock, totalBlocks, fetchCallback, flushCallback, nil,
	)
	assert.EqualValues(t, bytesPerBlock, cache.BytesPerBlock(), "wrong bytes per block")
	assert.EqualValues(t, totalBlocks, cache.TotalBlocks(), "wrong total blocks")
	assert.EqualValues(t, bytesPerBlock*totalBlocks, cache.Size(), "total size is wrong")
	return cache
}

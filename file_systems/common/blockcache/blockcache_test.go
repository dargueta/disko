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
}

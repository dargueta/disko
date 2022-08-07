// Package blockcache provides a block-oriented cache that can be used for
// providing a contiguous view of a single file system object scattered across
// the disk image.
//
// All block indexes begin at 0.

package blockcache

import (
	"fmt"

	"github.com/boljen/go-bitmap"
	c "github.com/dargueta/disko/drivers/common"
)

// FetchBlockCallback is a pointer to a function that writes the contents of a
// single block from the underlying storage into `buffer`. `buffer` is guaranteed
// to be the size of exactly one block.
type FetchBlockCallback func(blockIndex c.LogicalBlock, buffer []byte) error

// FlushBlockCallback is a pointer to a function that writes the contents of the
// given buffer to a block in the backing storage. `buffer` is guaranteed to be
// the size of exactly one block.
type FlushBlockCallback func(blockIndex c.LogicalBlock, buffer []byte) error

type BlockCache struct {
	loadedBlocks  bitmap.Bitmap
	dirtyBlocks   bitmap.Bitmap
	fetch         FetchBlockCallback
	flush         FlushBlockCallback
	bytesPerBlock uint
	totalBlocks   uint
	data          []byte
}

// New creates a new BlockCache.
func New(
	bytesPerBlock uint,
	totalBlocks uint,
	fetchCb FetchBlockCallback,
	flushCb FlushBlockCallback,
) *BlockCache {
	return &BlockCache{
		loadedBlocks:  bitmap.NewSlice(int(totalBlocks)),
		dirtyBlocks:   bitmap.NewSlice(int(totalBlocks)),
		data:          make([]byte, int(bytesPerBlock*totalBlocks)),
		fetch:         fetchCb,
		flush:         flushCb,
		bytesPerBlock: bytesPerBlock,
		totalBlocks:   totalBlocks,
	}
}

// BytesPerBlock returns the size of a single block, in bytes.
func (cache *BlockCache) BytesPerBlock() uint {
	return cache.bytesPerBlock
}

// TotalBlocks returns the size of the cache, in blocks. To change the size of
// the cache, use the Resize() function.
func (cache *BlockCache) TotalBlocks() uint {
	return cache.totalBlocks
}

func (cache *BlockCache) sizeToNumBlocks(size uint) uint {
	return (size + cache.bytesPerBlock - 1) / cache.bytesPerBlock
}

// checkBounds verifies that `bufferSize` bytes can be accessed in the cache
// starting from block `start`. If not, it returns an error describing the exact
// conditions. If no error would occur, this returns nil.
func (cache *BlockCache) checkBounds(start c.LogicalBlock, bufferSize uint) error {
	numBlocks := cache.sizeToNumBlocks(bufferSize)

	if uint(start)+numBlocks >= cache.totalBlocks {
		return fmt.Errorf(
			"can't access %d bytes (%d blocks) from block %d; range not in [0, %d)",
			bufferSize,
			numBlocks,
			start,
			cache.totalBlocks,
		)
	}
	return nil
}

// GetSlice returns a slice pointing to the cache's storage, beginning at block
// `start` and continuing for `count` blocks.
func (cache *BlockCache) GetSlice(
	start c.LogicalBlock,
	count uint,
) ([]byte, error) {
	err := cache.checkBounds(start, count*cache.bytesPerBlock)
	if err != nil {
		return nil, err
	}

	startOffset := uint(start) * cache.bytesPerBlock
	endOffset := startOffset + (count * cache.bytesPerBlock)
	return cache.data[startOffset:endOffset], nil
}

// loadBlockRange ensures that all blocks in the range [start, start + count) are
// present in the cache, and loads any missing ones from storage.
func (cache *BlockCache) loadBlockRange(start c.LogicalBlock, count uint) error {
	err := cache.checkBounds(start, count*cache.bytesPerBlock)
	if err != nil {
		return err
	}

	for blockIndex := int(start); uint(blockIndex) < uint(start)+count; blockIndex++ {
		// Skip if the block is in the cache. Since dirty blocks are present by
		// definition, we don't need to check `dirtyBlocks`.
		if cache.loadedBlocks.Get(blockIndex) {
			continue
		}

		buffer, err := cache.GetSlice(c.LogicalBlock(blockIndex), 1)
		if err != nil {
			return err
		}

		// Load the block from backing storage directly into the cache.
		err = cache.fetch(c.LogicalBlock(blockIndex), buffer)
		if err != nil {
			return fmt.Errorf(
				"failed to load block %d from source: %s",
				blockIndex,
				err.Error(),
			)
		}

		// Mark the block as present and clean.
		cache.loadedBlocks.Set(blockIndex, true)
		cache.dirtyBlocks.Set(blockIndex, false)
	}

	return nil
}

// flushBlockRange writes out all dirty blocks (and only dirty blocks) to the
// underlying storage and marks them as clean.
func (cache *BlockCache) flushBlockRange(start c.LogicalBlock, count uint) error {
	err := cache.checkBounds(start, count*cache.bytesPerBlock)
	if err != nil {
		return err
	}

	for blockIndex := int(start); uint(blockIndex) < uint(start)+count; blockIndex++ {
		// Skip if the block is clean. This also skips over blocks that aren't
		// loaded, since missing blocks are considered clean.
		if !cache.dirtyBlocks.Get(blockIndex) {
			continue
		}

		buffer, err := cache.GetSlice(c.LogicalBlock(blockIndex), 1)
		if err != nil {
			return err
		}

		// Write the block to the underlying storage.
		err = cache.flush(c.LogicalBlock(blockIndex), buffer)
		if err != nil {
			return fmt.Errorf(
				"failed to flush block %d to storage: %s", blockIndex, err.Error(),
			)
		}

		// Mark the flushed block as clean.
		cache.dirtyBlocks.Set(blockIndex, false)
	}

	return nil
}

// LoadAll ensures all missing blocks are loaded from storage into the cache.
func (cache *BlockCache) LoadAll() error {
	return cache.loadBlockRange(0, cache.totalBlocks)
}

// FlushAll flushes all dirty blocks from the cache into storage, and marks them
// as clean.
func (cache *BlockCache) FlushAll() error {
	return cache.flushBlockRange(0, cache.totalBlocks)
}

// Read fills `buffer` with data beginning at block `start`, loading any missing
// blocks first. `buffer` does not need to be an exact multiple of the size of
// one block.
//
// Attempting to read past the end of the cache will result in an error, and
// `buffer` will be left unmodified.
func (cache *BlockCache) Read(start c.LogicalBlock, buffer []byte) error {
	bufLen := uint(len(buffer))
	err := cache.checkBounds(start, bufLen)
	if err != nil {
		return err
	}

	numBlocks := cache.sizeToNumBlocks(bufLen)
	err = cache.loadBlockRange(start, numBlocks)
	if err != nil {
		return err
	}

	sourceData, err := cache.GetSlice(start, numBlocks)
	if err != nil {
		return err
	}

	copy(buffer, sourceData)
	return nil
}

// Write copies data into the cache from `buffer`, beginning at block `start`.
// All modified blocks are marked as dirty. `buffer` does not need to be an
// exact multiple of the size of one block.
//
// Attempting to write past the end of the cache will result in an error, and
// the cache will be left unmodified.
func (cache *BlockCache) Write(start c.LogicalBlock, buffer []byte) error {
	bufLen := uint(len(buffer))

	err := cache.checkBounds(start, bufLen)
	if err != nil {
		return err
	}

	totalBlocks := cache.sizeToNumBlocks(bufLen)
	targetByteSlice, err := cache.GetSlice(start, totalBlocks)
	if err != nil {
		return err
	}

	copy(targetByteSlice, buffer)

	// Mark all blocks we wrote to as present and dirty.
	for i := uint(0); i < totalBlocks; i++ {
		currentBlockIndex := int(c.LogicalBlock(i) + start)
		cache.loadedBlocks.Set(currentBlockIndex, true)
		cache.dirtyBlocks.Set(currentBlockIndex, true)
	}
	return nil
}

// Resize changes the number of blocks in the cache. Blocks are added to and
// removed from the end.
//
// If the cache size is increased, empty blocks are appended to the end of the
// slice. The unloaded blocks are treated as missing and clean, so calling
// FlushAll() will *not* write them out.
func (cache *BlockCache) Resize(newTotalBlocks uint) {
	newCacheData := make([]byte, newTotalBlocks*cache.bytesPerBlock)
	copy(newCacheData, cache.data)

	// Allocate new copies of the dirty/present bitmaps of the correct size.
	newDirtyBlocks := bitmap.NewSlice(int(newTotalBlocks))
	newLoadedBlocks := bitmap.NewSlice(int(newTotalBlocks))

	// Copy the old data over.
	copy(newDirtyBlocks, cache.dirtyBlocks)
	copy(newLoadedBlocks, cache.loadedBlocks)

	// Set the new values now that we've successfully allocated and copied all
	// the data.
	cache.data = newCacheData
	cache.dirtyBlocks = newDirtyBlocks
	cache.loadedBlocks = newLoadedBlocks
	cache.totalBlocks = newTotalBlocks
}

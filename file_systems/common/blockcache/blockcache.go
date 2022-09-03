// Package blockcache provides a block-oriented cache that can be used for
// providing a linear view of a single object scattered across discontiguous
// blocks in the disk image.
//
// All block indices begin at 0.

package blockcache

import (
	"fmt"
	"io"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko/errors"
	c "github.com/dargueta/disko/file_systems/common"
)

// FetchBlockCallback is a pointer to a function that writes the contents of a
// single block from the backing storage into `buffer`. The following guarantees
// apply:
//
// - `blockIndex` is in the range [0, TotalBlocks).
// - `buffer` is always BytesPerBlock bytes.
type FetchBlockCallback func(blockIndex c.LogicalBlock, buffer []byte) error

// FlushBlockCallback is a pointer to a function that writes the contents of the
// given buffer to a block in the backing storage. All restrictions and
// guarantees in [FetchBlockCallback] apply here too.
type FlushBlockCallback func(blockIndex c.LogicalBlock, buffer []byte) error

// ResizeCallback is a pointer to a function that is called to allocate or free
// blocks in the backing storage. It takes one argument, the new total number of
// blocks to occupy.
//
// The implementation of the callback can do anything so long as 1) it doesn't
// modify the data in the blocks; 2) at least the requested number of blocks are
// available once the function returns.
//
// Standard conditions for error codes:
//
//   - [errors.EFBIG]: Can't increase the size of the object because it would
//     exceed some technical limit.
//   - [errors.ENOSPC]: Can't increase the size of the object because there's no
//     space left on the volume.
//   - [errors.ENOTSUP]: The object can't be resized as a general rule. This is
//     mostly only seen in systems with fixed-size directories, like FAT8/12/16.
type ResizeCallback func(newTotalBlocks c.LogicalBlock) error

type BlockCache struct {
	loadedBlocks  bitmap.Bitmap
	dirtyBlocks   bitmap.Bitmap
	fetch         FetchBlockCallback
	flush         FlushBlockCallback
	resize        ResizeCallback
	bytesPerBlock uint
	totalBlocks   uint
	data          []byte
}

// New creates a new BlockCache.
//
// There are three callback functions:
//
//   - `fetchCb` reads a single block from the backing storage.
//   - `flushCb` writes a single block to the backing storage.
//   - `resizeCb` resizes the backing storage to a given number of blocks. If
//     nil is passed for this argument, a stub function is provided that always
//     returns an error with code [errors.ENOTSUP].
func New(
	bytesPerBlock uint,
	totalBlocks uint,
	fetchCb FetchBlockCallback,
	flushCb FlushBlockCallback,
	resizeCb ResizeCallback,
) *BlockCache {
	if resizeCb == nil {
		resizeCb = func(newTotalBlocks c.LogicalBlock) error {
			return errors.NewWithMessage(
				errors.ENOTSUP,
				fmt.Sprintf(
					"resizing is not supported; size fixed at %d bytes",
					bytesPerBlock*totalBlocks,
				),
			)
		}
	}

	return &BlockCache{
		loadedBlocks:  bitmap.NewSlice(int(totalBlocks)),
		dirtyBlocks:   bitmap.NewSlice(int(totalBlocks)),
		data:          make([]byte, int(bytesPerBlock*totalBlocks)),
		fetch:         fetchCb,
		flush:         flushCb,
		resize:        resizeCb,
		bytesPerBlock: bytesPerBlock,
		totalBlocks:   totalBlocks,
	}
}

// WrapStream creates a [BlockCache] that wraps any [io.ReadWriteSeeker],
// optionally forbidding resizing the stream. To support resizing, `stream` must
// implement [common.Truncator], equivalent to [os.File.Truncate].
func WrapStream(
	stream io.ReadWriteSeeker,
	bytesPerBlock uint,
	totalBlocks uint,
	allowResize bool,
) *BlockCache {
	// This function performs the function of the read and write callbacks. It's
	// put into one function because reading and writing differ only by a single
	// method call on the stream.
	runCb := func(block c.LogicalBlock, buffer []byte, read bool) error {
		err := seekToBlock(stream, block, c.LogicalBlock(totalBlocks), bytesPerBlock)
		if err != nil {
			return err
		}

		if read {
			_, err = stream.Read(buffer)
		} else {
			_, err = stream.Write(buffer)
		}

		if err != nil && err != io.EOF {
			return err
		}
		return nil
	}

	fetchCb := func(block c.LogicalBlock, buffer []byte) error {
		return runCb(block, buffer, true)
	}

	flushCb := func(block c.LogicalBlock, buffer []byte) error {
		return runCb(block, buffer, false)
	}

	var resizeCb ResizeCallback
	_, streamHasTruncate := stream.(c.Truncator)

	if allowResize && streamHasTruncate {
		// Resizing the stream is allowed.
		resizeCb = func(newTotalBlocks c.LogicalBlock) error {
			truncator := stream.(c.Truncator)
			return truncator.Truncate(int64(newTotalBlocks) * int64(bytesPerBlock))
		}
	} else {
		// Resizing is not allowed, either because the caller forbade it or the
		// stream doesn't support it. Note we don't return ENOSYS here because
		// the functionality *is* supported by Disko, but not this specific
		// stream.
		resizeCb = func(newTotalBlocks c.LogicalBlock) error {
			return errors.New(errors.ENOTSUP)
		}
	}

	return New(bytesPerBlock, totalBlocks, fetchCb, flushCb, resizeCb)
}

// seekToBlock sets the stream pointer for a stream to the offset of a block.
func seekToBlock(stream io.Seeker, block, totalBlocks c.LogicalBlock, bytesPerBlock uint) error {
	if block >= totalBlocks {
		return errors.NewWithMessage(
			errors.EINVAL,
			fmt.Sprintf(
				"invalid block number: %d not in range [0, %d)",
				block,
				totalBlocks,
			),
		)
	}

	blockOffset := int64(block) * int64(bytesPerBlock)
	_, err := stream.Seek(blockOffset, io.SeekStart)
	return err
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

// Size gives the size of the cache, in bytes (not blocks!).
func (cache *BlockCache) Size() int64 {
	return int64(cache.bytesPerBlock) * int64(cache.totalBlocks)
}

// LengthToNumBlocks gives the minimum number of blocks required to hold the
// given number of bytes.
func (cache *BlockCache) LengthToNumBlocks(size uint) uint {
	return (size + cache.bytesPerBlock - 1) / cache.bytesPerBlock
}

// checkBounds verifies that `bufferSize` bytes can be accessed in the cache
// starting from block `start`. If not, it returns an error describing the exact
// conditions. If no error would occur, this returns nil.
func (cache *BlockCache) checkBounds(start c.LogicalBlock, bufferSize uint) error {
	numBlocks := cache.LengthToNumBlocks(bufferSize)

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
//
// If the returned slice is modified, the modified blocks MUST be marked as
// dirty.
func (cache *BlockCache) GetSlice(
	start c.LogicalBlock,
	count uint,
) ([]byte, error) {
	err := cache.loadBlockRange(start, count)
	if err != nil {
		return nil, err
	}

	startOffset := uint(start) * cache.bytesPerBlock
	endOffset := startOffset + (count * cache.bytesPerBlock)
	return cache.data[startOffset:endOffset], nil
}

// Data returns a slice of the entire cache's data. This requires loading all
// blocks not yet in the cache, so it may incur a one-time performance penalty
// for large files or with inefficient driver implementations.
//
// If the returned slice is modified, the modified blocks MUST be marked as
// dirty.
func (cache *BlockCache) Data() ([]byte, error) {
	err := cache.LoadAll()
	if err != nil {
		return nil, err
	}
	return cache.data[:], nil
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

// Flush flushes all dirty blocks from the cache into storage, and marks them
// as clean.
func (cache *BlockCache) Flush() error {
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

	numBlocks := cache.LengthToNumBlocks(bufLen)
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

	totalBlocks := cache.LengthToNumBlocks(bufLen)
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
// If the cache size is increased, zeroed-out blocks are appended to the end of
// the slice. These new blocks are treated as dirty, so flushing the cache will
// write them out.
func (cache *BlockCache) Resize(newTotalBlocks uint) error {
	err := cache.resize(c.LogicalBlock(newTotalBlocks))
	if err != nil {
		return err
	}

	newCacheData := make([]byte, uint(newTotalBlocks)*cache.bytesPerBlock)
	copy(newCacheData, cache.data)

	// Allocate new copies of the dirty/present bitmaps of the correct size.
	newDirtyBlocks := bitmap.Bitmap(bitmap.NewSlice(int(newTotalBlocks)))
	newLoadedBlocks := bitmap.Bitmap(bitmap.NewSlice(int(newTotalBlocks)))

	// Copy the old data over.
	copy(newDirtyBlocks, cache.dirtyBlocks)
	copy(newLoadedBlocks, cache.loadedBlocks)

	// If we added any blocks, mark them as dirty. Since memory is zeroed out
	// when allocating, this means that if the data isn't modified we'll write
	// out zeroed blocks. If we didn't mark them dirty, they wouldn't get
	// written, and we could end up with trailing blocks filled with uninitialized
	// data.
	for i := cache.totalBlocks; i < newTotalBlocks; i++ {
		newDirtyBlocks.Set(int(i), true)
		newLoadedBlocks.Set(int(i), true)
	}

	// Set the new values now that we've successfully allocated and copied all
	// the data.
	cache.data = newCacheData
	cache.dirtyBlocks = newDirtyBlocks
	cache.loadedBlocks = newLoadedBlocks
	cache.totalBlocks = newTotalBlocks
	return nil
}

// MarkBlockRangeDirty marks a range of blocks as modified. They will be written
// out to the backing storage on the next call to [FlushAll].
func (cache *BlockCache) MarkBlockRangeDirty(
	start c.LogicalBlock,
	count uint,
) error {
	err := cache.checkBounds(start, count*cache.bytesPerBlock)
	if err != nil {
		return err
	}

	for i := uint(0); i < count; i++ {
		// FIXME(dargueta): We can end up with integer overflow here
		bitIndex := int(start) + int(i)
		cache.dirtyBlocks.Set(bitIndex, true)
		cache.loadedBlocks.Set(bitIndex, true)
	}
	return nil
}

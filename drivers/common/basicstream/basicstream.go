// Package basicstream implements a basic file-like abstraction around a
// block-oriented cache.

package basicstream

import (
	"fmt"
	"io"
	"math"

	c "github.com/dargueta/disko/drivers/common"
	"github.com/dargueta/disko/drivers/common/blockcache"
)

// BasicStream is a file-like wrapper around a BlockCache that emulates a
// subset of the functionality provided by an `os.File` instance.
type BasicStream struct {
	// Interfaces
	io.Closer
	io.ReaderAt
	io.ReadWriteSeeker
	io.StringWriter
	io.WriterAt

	// Fields
	size     int64
	position int64
	data     *blockcache.BlockCache
}

// New creates a BasicStream on top of a block cache. The `size` argument gives
// the exact size of the stream, in bytes. The only requirement for this is that
// it must be between 0 and `data.Size()` (inclusive).
func New(size int64, data *blockcache.BlockCache) (*BasicStream, error) {
	maxSize := data.Size()
	if size < 0 || size > maxSize {
		return nil, fmt.Errorf(
			"invalid stream size: %d not in the range [0, %d]",
			size,
			maxSize,
		)
	}

	return &BasicStream{
		size:     size,
		position: 0,
		data:     data,
	}, nil
}

func (stream *BasicStream) convertLinearAddr(offset int64) (c.LogicalBlock, uint) {
	bytesPerBlock := int64(stream.data.BytesPerBlock())
	return c.LogicalBlock(offset / bytesPerBlock), uint(offset % bytesPerBlock)
}

// Close writes out all pending changes to the underlying storage. The stream
// should not be used for I/O operations after calling this method.
func (stream *BasicStream) Close() error {
	return stream.Sync()
}

func (stream *BasicStream) Read(buffer []byte) (int, error) {
	totalRead, err := stream.ReadAt(buffer, stream.position)
	stream.position += int64(totalRead)
	return totalRead, err
}

func (stream *BasicStream) ReadAt(buffer []byte, offset int64) (int, error) {
	bufLen := int64(len(buffer))

	// Clamp the number of bytes to read to whichever is smaller; the length of
	// the buffer or the end of the file.
	var numBytesToRead int64
	if offset >= stream.size {
		return 0, io.EOF
	} else if offset+bufLen >= stream.size {
		numBytesToRead = stream.size - offset
	} else {
		numBytesToRead = bufLen
	}

	firstBlock, firstBlockOffset := stream.convertLinearAddr(offset)
	lastBlock, _ := stream.convertLinearAddr(offset + numBytesToRead)

	sourceData, err := stream.data.GetSlice(
		firstBlock,
		uint(lastBlock-firstBlock)+1,
	)
	if err != nil {
		return 0, err
	}

	copy(buffer, sourceData[firstBlockOffset:firstBlockOffset+uint(numBytesToRead)])

	if numBytesToRead < bufLen {
		err = io.EOF
	}
	return int(numBytesToRead), err
}

// Seek resets the stream pointer to `offset` bytes from the origin specified in
// `whence`. It must be one of `io.SeekStart`, `io.SeekCurrent`, or `io.SeekEnd`.
//
// Seeking past the end of the file is possible; the file will automatically be
// resized upon the first write. Attempting to read past the end of the file
// returns no data.
func (stream *BasicStream) Seek(offset int64, whence int) (int64, error) {
	var absoluteOffset int64

	switch whence {
	case io.SeekStart:
		absoluteOffset = offset
	case io.SeekCurrent:
		absoluteOffset += offset
	case io.SeekEnd:
		absoluteOffset = stream.size + offset
	default:
		return stream.position, fmt.Errorf("invalid seek origin: %d", whence)
	}

	if absoluteOffset < 0 {
		return stream.position,
			fmt.Errorf(
				"result of Seek(offset=%d, whence=%d) is negative",
				offset,
				whence,
			)
	}

	stream.position = absoluteOffset
	return absoluteOffset, nil
}

// Size returns the size of the file, in bytes.
func (stream *BasicStream) Size() int64 {
	return stream.size
}

// Sync writes out all pending changes to the backing storage. After calling this,
// all loaded blocks will be marked clean.
func (stream *BasicStream) Sync() error {
	return stream.data.FlushAll()
}

// Tell returns the current stream position. It's a more concise way of calling
// `Seek(0, io.SeekCurrent)`.
func (stream *BasicStream) Tell() int64 {
	return stream.position
}

// Truncate resizes the stream to the given number of bytes but does not move
// the stream pointer.
func (stream *BasicStream) Truncate(size int64) error {
	if size < 0 {
		return fmt.Errorf("truncate failed: %d is not a valid file size", size)
	} else if uint64(size) > math.MaxUint {
		return fmt.Errorf("truncate failed: new file size %d is too large", size)
	}
	newTotalBlocks := stream.data.LengthToNumBlocks(uint(size))

	err := stream.data.Resize(newTotalBlocks)
	if err != nil {
		return err
	}

	stream.size = size
	return nil
}

func (stream *BasicStream) Write(buffer []byte) (int, error) {
	totalWritten, err := stream.WriteAt(buffer, stream.position)
	stream.position += int64(totalWritten)
	return totalWritten, err
}

func (stream *BasicStream) WriteAt(buffer []byte, offset int64) (int, error) {
	bufLen := int64(len(buffer))

	startBlock, startOffset := stream.convertLinearAddr(offset)
	lastBlock, _ := stream.convertLinearAddr(offset + bufLen)

	// If we're going to end up writing past the end of the stream we need to
	// grow the file first.
	if uint(lastBlock) >= stream.data.TotalBlocks() {
		err := stream.Truncate(offset + bufLen)
		if err != nil {
			return 0, err
		}
	}

	targetSlice, err := stream.data.GetSlice(startBlock, uint(lastBlock)+1)
	if err != nil {
		return 0, err
	}

	copy(targetSlice[startOffset:], buffer)
	return len(buffer), nil
}

// WriteString writes a string to the stream.
func (stream *BasicStream) WriteString(s string) (int, error) {
	return stream.Write([]byte(s))
}

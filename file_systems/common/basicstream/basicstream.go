// Package basicstream implements a basic file-like abstraction around a
// block-oriented cache.

package basicstream

import (
	"fmt"
	"io"
	"math"

	"github.com/dargueta/disko"
	c "github.com/dargueta/disko/file_systems/common"
	"github.com/dargueta/disko/file_systems/common/blockcache"
)

// BasicStream is a file-like wrapper around a BlockCache that emulates a
// subset of the functionality provided by an [os.File] instance.
type BasicStream struct {
	size     int64
	position int64
	data     *blockcache.BlockCache
	ioFlags  disko.IOFlags
}

// New creates a [BasicStream] on top of a block cache. The `size` argument gives
// the exact size of the stream, in bytes. The only requirement for this is that
// it must be between 0 and `data.Size()` (inclusive).
//
// All relevant behaviors of [disko.IOFlags] are implemented. In particular:
//
//   - Read/write permissions are enforced, e.g. attempting to write a file
//     created with [disko.O_RDONLY] will fail with [disko.ErrNotPermitted].
//   - [disko.O_APPEND], [disko.O_SYNC], and [disko.O_TRUNC] are obeyed.
func New(
	size int64,
	data *blockcache.BlockCache,
	flags disko.IOFlags,
) (*BasicStream, error) {
	maxSize := data.Size()
	if size < 0 || size > maxSize {
		return nil, fmt.Errorf(
			"invalid stream size: %d not in the range [0, %d]",
			size,
			maxSize,
		)
	}

	stream := &BasicStream{
		size:     size,
		position: 0,
		data:     data,
		ioFlags:  flags,
	}

	if flags.Truncate() {
		return stream, stream.Truncate(0)
	}
	return stream, nil
}

// convertLinearAddr converts a byte offset into a block+block offset pair. It
// disregards the actual size of the stream, so it's possible (by design) to
// generate offsets beyond the end of the stream. `offs` is guaranteed to be in
// the range [0, BytesPerBlock).
func (stream *BasicStream) convertLinearAddr(offset int64) (blk c.LogicalBlock, offs uint) {
	bytesPerBlock := int64(stream.data.BytesPerBlock())
	blk = c.LogicalBlock(offset / bytesPerBlock)
	offs = uint(offset % bytesPerBlock)
	return
}

// Close writes out all pending changes to the underlying storage. The stream
// should not be used for I/O operations after calling this method.
func (stream *BasicStream) Close() error {
	return stream.Sync()
}

// Read implements [io.Reader].
func (stream *BasicStream) Read(buffer []byte) (int, error) {
	totalRead, err := stream.ReadAt(buffer, stream.position)
	if err == nil {
		stream.position += int64(totalRead)
	}
	return totalRead, err
}

// ReadAt implements [io.ReaderAt].
func (stream *BasicStream) ReadAt(buffer []byte, offset int64) (int, error) {
	if !stream.ioFlags.Read() {
		return 0, disko.ErrNotPermitted
	}

	// The address computations below assume we're reading at least one byte. If
	// the target buffer is empty, we have nothing further to do.
	bufLen := int64(len(buffer))
	if bufLen == 0 {
		return 0, nil
	}

	// Clamp the number of bytes to read to whichever is smaller; the length of
	// the buffer or the number of bytes remaining in the file.
	var numBytesToRead int64
	if offset >= stream.size {
		return 0, io.EOF
	} else if offset+bufLen >= stream.size {
		numBytesToRead = stream.size - offset
	} else {
		numBytesToRead = bufLen
	}

	firstBlock, startOffset := stream.convertLinearAddr(offset)
	lastBlock, _ := stream.convertLinearAddr(offset + numBytesToRead - 1)

	sourceData, err := stream.data.GetSlice(
		firstBlock,
		uint(lastBlock-firstBlock+1),
	)
	if err != nil {
		return 0, err
	}

	copy(buffer, sourceData[startOffset:startOffset+uint(numBytesToRead)])

	if numBytesToRead < bufLen {
		err = io.EOF
	}
	return int(numBytesToRead), err
}

// ReadFrom implements [io.ReaderFrom].
func (stream *BasicStream) ReadFrom(r io.Reader) (n int64, err error) {
	if !stream.ioFlags.Write() {
		return 0, disko.ErrNotPermitted
	}

	// If the argument is another BasicStream, make the read buffer be exactly
	// one block in size to simplify reading. Otherwise, default to 512 bytes.
	otherStream, ok := r.(*BasicStream)
	var blockSize int
	if ok {
		blockSize = int(otherStream.data.BytesPerBlock())
	} else {
		blockSize = 512
	}

	buffer := make([]byte, blockSize)

	totalBytesRead := int64(0)
	for {
		lastReadSize, readErr := r.Read(buffer)
		totalBytesRead += int64(lastReadSize)

		_, writeErr := stream.Write(buffer[:lastReadSize])
		if readErr == io.EOF {
			return totalBytesRead, nil
		} else if readErr != nil {
			return totalBytesRead, readErr
		} else if writeErr != nil {
			return totalBytesRead, writeErr
		}
	}
}

// Seek resets the stream pointer to `offset` bytes from the origin specified in
// `whence`. It must be one of [io.SeekStart], [io.SeekCurrent], or [io.SeekEnd].
//
// Seeking past the end of the file is possible; the file will automatically be
// resized upon the first write past the end. Attempting to read past the end of
// the file returns no data.
//
// If the stream was created with the `O_APPEND` flag, seeking will succeed, but
// any write operation will automatically reposition the stream pointer to the
// end of the file before writing.
func (stream *BasicStream) Seek(offset int64, whence int) (int64, error) {
	var absoluteOffset int64

	switch whence {
	case io.SeekStart:
		absoluteOffset = offset
	case io.SeekCurrent:
		absoluteOffset = stream.position + offset
	case io.SeekEnd:
		absoluteOffset = stream.size + offset
	default:
		return stream.position, fmt.Errorf("invalid seek origin: %d", whence)
	}

	if absoluteOffset < 0 {
		return stream.position,
			fmt.Errorf(
				"result of Seek(offset=%d, whence=%d) is negative: %d",
				offset,
				whence,
				absoluteOffset,
			)
	}

	stream.position = absoluteOffset
	return absoluteOffset, nil
}

// Size returns the size of the file, in bytes.
func (stream *BasicStream) Size() int64 {
	return stream.size
}

// Sync writes out all pending changes to the backing storage. After calling
// this, all loaded blocks will be marked clean.
func (stream *BasicStream) Sync() error {
	return stream.data.Flush()
}

// Tell returns the current stream position. It's slightly more efficient than
// `Seek(0, io.SeekCurrent)`.
func (stream *BasicStream) Tell() int64 {
	return stream.position
}

// Truncate resizes the stream to the given number of bytes but doesn't move the
// stream pointer.
func (stream *BasicStream) Truncate(size int64) error {
	if !stream.ioFlags.Write() {
		return disko.ErrNotPermitted
	}

	if size < 0 {
		return disko.ErrInvalidArgument.WithMessage(
			fmt.Sprintf("truncate failed: %d is not a valid file size", size),
		)
	} else if uint64(size) > math.MaxUint {
		return disko.ErrFileTooLarge.WithMessage(
			fmt.Sprintf("truncate failed: new file size %d is too large", size),
		)
	}
	newTotalBlocks := stream.data.GetMinBlocksForSize(uint(size))

	err := stream.data.Resize(newTotalBlocks)
	if err != nil {
		return err
	}

	stream.size = size

	if stream.ioFlags.Synchronous() {
		return stream.Sync()
	}
	return nil
}

// Write implements [io.Writer].
func (stream *BasicStream) Write(buffer []byte) (int, error) {
	var err error

	if !stream.ioFlags.Write() {
		return 0, disko.ErrNotPermitted
	}

	// Force the stream pointer to the end of the file if O_APPEND was set.
	if stream.ioFlags.Append() {
		_, err = stream.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
	}

	// NB we must call implWriteAt, not WriteAt, since WriteAt fails if the
	// O_APPEND flag is set.
	totalWritten, err := stream.implWriteAt(buffer, stream.position)
	stream.position += int64(totalWritten)
	return totalWritten, err
}

// implWriteAt implements the bulk of WriteAt with the exception that it doesn't
// check for the O_APPEND flag.
func (stream *BasicStream) implWriteAt(buffer []byte, offset int64) (int, error) {
	if !stream.ioFlags.Write() {
		return 0, disko.ErrNotPermitted
	}

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

	if stream.ioFlags.Synchronous() {
		return len(buffer), stream.Sync()
	}
	return len(buffer), nil
}

// WriteAt implements [io.WriterAt] It is an error to use this function if the
// stream was created with the [disko.O_APPEND] flag.
func (stream *BasicStream) WriteAt(buffer []byte, offset int64) (int, error) {
	if stream.ioFlags.Append() {
		return 0, disko.ErrNotPermitted
	}
	return stream.implWriteAt(buffer, offset)
}

// WriteString implements [io.StringWriter].
func (stream *BasicStream) WriteString(s string) (int, error) {
	return stream.Write([]byte(s))
}

// WriteTo implements [io.WriterTo].
func (stream *BasicStream) WriteTo(w io.Writer) (n int64, err error) {
	buffer := make([]byte, stream.data.BytesPerBlock())
	totalWritten := int64(0)

	for {
		blockSize, err := stream.Read(buffer)

		// Always write the data we've read in regardless of whether an error
		// occurred or not.
		if blockSize > 0 {
			w.Write(buffer[:blockSize])
			totalWritten += int64(blockSize)
		}

		// If we hit EOF, we're done. Any other error is fatal.
		if err == io.EOF {
			return totalWritten, nil
		} else if err != nil {
			return totalWritten, err
		}
	}
}

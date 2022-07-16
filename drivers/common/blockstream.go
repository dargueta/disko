package common

import (
	"fmt"
	"io"

	"github.com/dargueta/disko"
)

type BlockID uint
type BlockData []byte

// BlockStream is an abstraction layer around a stream to make it look like a
// block stream, e.g. a file that can only be read from or written to in
// multiples of its fundamental unit, a "block".
//
// The exposed fields are for informational purposes only and should never be
// changed.
type BlockStream struct {
	// BytesPerBlock gives the size of a block on this device, in bytes. All reads
	// and writes must be done in integer multiples of this size.
	BytesPerBlock uint
	// TotalBlocks is the total number of blocks in this stream.
	TotalBlocks uint
	// StartOffset is an offset from the beginning of the stream, in bytes, that
	// will be considered the beginning of block 0 for the device. This is useful
	// for skipping over MBRs or other volumes stored on the same image.
	StartOffset int64
	stream      io.ReadWriteSeeker
}

func NewBlockStream(
	stream io.ReadWriteSeeker, totalBlocks uint, blockSize uint, startOffset int64,
) BlockStream {
	return BlockStream{
		StartOffset:   startOffset,
		BytesPerBlock: blockSize,
		TotalBlocks:   totalBlocks,
		stream:        stream,
	}
}

// DetermineBlockCount gives the total number of blocks in a stream, rounded down
// to the nearest block.
func DetermineBlockCount(stream io.Seeker, blockSize uint) (uint, error) {
	offset, err := stream.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	return uint(offset / int64(blockSize)), nil
}

// NewSectorDevice is a constructor that creates a new BlockDevice with 512-byte
// blocks and starts from an offset of 0.
func NewBasicBlockStream(stream io.ReadWriteSeeker, totalBlocks uint) BlockStream {
	return NewBlockStream(stream, totalBlocks, 512, 0)
}

// BlockIDToFileOffset converts a block ID into a byte offset into the backing
// I/O stream.
func (device *BlockStream) BlockIDToFileOffset(blockID BlockID) (int64, error) {
	if uint(blockID) >= device.TotalBlocks {
		return -1,
			fmt.Errorf(
				"invalid block ID %d: not in range [0, %d)",
				blockID,
				device.TotalBlocks)
	}
	return device.StartOffset + (int64(blockID) * int64(device.BytesPerBlock)), nil
}

// CheckIOBounds checks to see if `dataLength` bytes can be read from or written
// to the block stream, starting at blockID. If the bounds check fails, it returns
// an error indicating exactly what went wrong.
func (device *BlockStream) CheckIOBounds(blockID BlockID, dataLength uint) error {
	if uint(blockID) >= device.TotalBlocks {
		return fmt.Errorf(
			"invalid block ID %d: not in range [0, %d)",
			blockID,
			device.TotalBlocks)
	}

	if dataLength%device.BytesPerBlock != 0 {
		return fmt.Errorf(
			"data must be a multiple of the block size (%d B), got %d (remainder %d)",
			device.BytesPerBlock,
			dataLength,
			dataLength%device.BytesPerBlock)
	}

	dataSizeInBlocks := dataLength / device.BytesPerBlock
	if uint(blockID)+dataSizeInBlocks >= device.TotalBlocks {
		return fmt.Errorf(
			"block %d plus %d blocks of data extends past end of image",
			blockID,
			dataSizeInBlocks)
	}

	return nil
}

// seekToBlock positions the stream pointer at the byte offset where the given
// block starts.
func (device *BlockStream) seekToBlock(blockID BlockID) error {
	offset, err := device.BlockIDToFileOffset(blockID)
	if err != nil {
		return err
	}
	_, err = device.stream.Seek(offset, io.SeekStart)
	return err
}

// Read reads `count` whole blocks starting from `blockID`.
func (device *BlockStream) Read(blockID BlockID, count uint) ([]byte, error) {
	err := device.CheckIOBounds(blockID, count*device.BytesPerBlock)
	if err != nil {
		return nil, err
	}

	err = device.seekToBlock(blockID)
	if err != nil {
		return nil, err
	}

	readSize := device.BytesPerBlock * count
	buffer := make([]byte, device.BytesPerBlock*count)
	bytesRead, err := device.stream.Read(buffer)
	if bytesRead < int(readSize) || err != nil {
		return nil, err
	}
	return buffer, nil
}

// Write writes data to the block device. `data` must be a multiple of the block
// size.
func (device *BlockStream) Write(blockID BlockID, data []byte) error {
	err := device.CheckIOBounds(blockID, uint(len(data)))
	if err != nil {
		return err
	}

	err = device.seekToBlock(blockID)
	if err != nil {
		return err
	}

	_, err = device.stream.Write(data)
	return err
}

func (device *BlockStream) Resize(newNumBlocks uint) error {
	if device.TotalBlocks == newNumBlocks {
		return nil
	}

	var err error

	// If the image is smaller than the requested new size, write the number of
	// missing blocks, filled with nulls.
	if device.TotalBlocks < newNumBlocks {
		numMissing := newNumBlocks - device.TotalBlocks
		nullData := make([]byte, device.BytesPerBlock*numMissing)
		_, err = device.stream.Write(nullData)
		return err
	} else {
		// The image is larger than the requested new size. Attempt to truncate
		// it.
		truncator, ok := device.stream.(disko.Truncator)
		if ok {
			err = truncator.Truncate(int64(newNumBlocks * device.BytesPerBlock))
		} else {
			// The stream doesn't have a compatible Truncate() function, so we
			// can't shrink the image.
			err = fmt.Errorf(
				"can't resize image from %d blocks to %d: the underlying stream"+
					" doesn't provide a way to shrink it",
				device.TotalBlocks,
				newNumBlocks,
			)
		}
	}

	if err == nil {
		device.TotalBlocks = newNumBlocks
		return nil
	}
	return err
}

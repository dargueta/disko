package common

import (
	"fmt"
	"io"
)

type BlockID uint
type BlockData []byte

// BlockDevice is an abstraction layer around a stream to make it look like a
// block stream, e.g. a file that can only be read from or written to in
// multiples of its fundamental unit, a "block".
//
// The exposed fields are for informational purposes only and should never be
// changed.
type BlockDevice struct {
	// BlockSize gives the size of a block on this device, in bytes. All reads
	// and writes must be done in integer multiples of this size.
	BlockSize uint
	// TotalBlocksThe total number of blocks in this stream.
	TotalBlocks uint
	// StartOffset is an offset from the beginning of the stream, in bytes, that
	// will be considered the beginning of block 0 for the device. This is useful
	// for skipping over MBRs or other volumes stored on the same image.
	StartOffset int64
	stream      *io.Seeker
}

func NewBlockDevice(stream *io.Seeker, totalBlocks uint, blockSize uint, startOffset int64) BlockDevice {
	return BlockDevice{
		StartOffset: startOffset,
		BlockSize:   blockSize,
		TotalBlocks: totalBlocks,
		stream:      stream,
	}
}

// NewSectorDevice is a constructor that creates a new BlockDevice with 512-byte
// blocks and starts from an offset of 0.
func NewSectorDevice(stream *io.Seeker, totalBlocks uint) BlockDevice {
	return NewBlockDevice(stream, totalBlocks, 512, 0)
}

func (device *BlockDevice) BlockIDToFileOffset(blockID BlockID) (int64, error) {
	if uint(blockID) >= device.TotalBlocks {
		return -1,
			fmt.Errorf(
				"invalid block ID %d: not in range [0, %d)",
				blockID,
				device.TotalBlocks)
	}
	return device.StartOffset + (int64(blockID) * int64(device.BlockSize)), nil
}

func (device *BlockDevice) CheckIOBounds(blockID BlockID, dataLength uint) error {
	if uint(blockID) >= device.TotalBlocks {
		return fmt.Errorf(
			"invalid block ID %d: not in range [0, %d)",
			blockID,
			device.TotalBlocks)
	}

	if dataLength%device.BlockSize != 0 {
		return fmt.Errorf(
			"data must be a multiple of the block size (%d B), got %d (remainder %d)",
			device.BlockSize,
			dataLength,
			dataLength%device.BlockSize)
	}

	dataSizeInBlocks := dataLength / device.BlockSize
	if uint(blockID)+dataSizeInBlocks >= device.TotalBlocks {
		return fmt.Errorf(
			"block %d plus %d blocks of data extends past end of image",
			blockID,
			dataSizeInBlocks)
	}

	return nil
}

func (device *BlockDevice) seekToBlock(blockID BlockID) error {
	offset, err := device.BlockIDToFileOffset(blockID)
	if err != nil {
		return err
	}
	_, err = (*device.stream).Seek(offset, io.SeekStart)
	return err
}

func (device *BlockDevice) Read(blockID BlockID, count uint) ([]byte, error) {
	stream := (*device.stream).(io.ReadSeeker)

	err := device.CheckIOBounds(blockID, count*device.BlockSize)
	if err != nil {
		return nil, err
	}

	err = device.seekToBlock(blockID)
	if err != nil {
		return nil, err
	}

	readSize := device.BlockSize * count
	buffer := make([]byte, device.BlockSize*count)
	bytesRead, err := stream.Read(buffer)
	if bytesRead < int(readSize) || err != nil {
		return nil, err
	}
	return buffer, nil
}

// Write writes data to the block device. `data` must be a multiple of the block
// size.
func (device *BlockDevice) Write(blockID BlockID, data []byte) error {
	stream := (*device.stream).(io.WriteSeeker)

	err := device.CheckIOBounds(blockID, uint(len(data)))
	if err != nil {
		return err
	}

	err = device.seekToBlock(blockID)
	if err != nil {
		return err
	}

	_, err = stream.Write(data)
	return err
}

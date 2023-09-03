// Package common contains definitions of fundamental types and functions used
// across multiple file system implementations.
package common

import (
	"math"
)

type LogicalBlock uint64
type PhysicalBlock uint64
type BlockIndex uint64

const InvalidLogicalBlock = LogicalBlock(math.MaxUint64)
const InvalidPhysicalBlock = PhysicalBlock(math.MaxUint64)
const InvalidBlockOffset = PhysicalBlock(math.MaxUint64)

// Truncator is an interface for objects that support a Truncate() method. This
// method must behave just like [os.File.Truncate].
type Truncator interface {
	Truncate(size int64) error
}

// A BlockDevice represents a resource that can be accessed like a file, but
// with fixed-size groups of bytes ("blocks") rather than individual bytes.
type BlockDevice interface {
	// BytesPerBlock returns the size of a single block, in bytes.
	BytesPerBlock() uint

	// TotalBlocks returns the size of the device, in blocks. To change the size
	// of the device, use [BlockDeviceResizer.Resize] if available.
	TotalBlocks() uint

	// Size gives the size of the device, in bytes (not blocks!). This must be
	// strictly more than `BytesPerBlock * (TotalBlocks - 1)` and less than or
	// equal to `BytesPerBlock * TotalBlocks`.
	Size() int64

	// GetMinBlocksForSize gives the minimum number of blocks required to hold
	// the given number of bytes.
	GetMinBlocksForSize(size uint) uint
}

type BlockDeviceReader interface {
	// ReadAt reads data beginning at the given logical block (indexed from 0)
	// into the buffer. `buffer` must be an integral multiple of the block size.
	ReadAt(buffer []byte, start LogicalBlock) (int, error)
}

type BlockDeviceWriter interface {
	// WriteAt writes data beginning at the given logical block (indexed from 0)
	// into the device. `buffer` must be an integral multiple of the block size.
	WriteAt(buffer []byte, start LogicalBlock) (int, error)
	Flush() error
}

type BlockDeviceReaderWriter interface {
	BlockDeviceReader
	BlockDeviceWriter
}

// A BlockDeviceResizer allows resizing a [BlockDevice].
type BlockDeviceResizer interface {
	// Resize resizes a block device to the given number of blocks. If the size
	// is increased, new, null-filled blocks are appended to the end. If the
	// size decreases, blocks are removed from the end.
	//
	// Because this operates at the block device level, resizing can damage the
	// file system on the device (if any).
	Resize(newTotalBlocks uint) error
}

// A DiskImage is a [BlockDevice] that supports random-access reading.
type DiskImage interface {
	BlockDevice
	BlockDeviceReader
}

// A WritableDiskImage is a [DiskImage] that supports both random-access reading
// and writing.
type WritableDiskImage interface {
	DiskImage
	BlockDeviceWriter
}

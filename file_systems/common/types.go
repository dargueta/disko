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
	// The blocks are automatically marked as dirty.
	WriteAt(buffer []byte, start LogicalBlock) (int, error)

	// Flush writes out all pending changes to the underlying storage.
	Flush() error

	// MarkBlockRangeDirty marks a range of blocks as needing to be flushed,
	// regardless of whether they've been written to or not. If this device does
	// no buffering, implementations must not return an error.
	MarkBlockRangeDirty(start LogicalBlock, length uint) error

	// MarkBlockRangeClean marks a range of blocks as unmodified and not needing
	// to be flushed. In that sense, it's the inverse of [MarkBlockRangeDirty].
	// If the device does no buffering, implementations must not return an error.
	MarkBlockRangeClean(start LogicalBlock, length uint) error
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
	// Because this operates at the block level, resizing may damage a file
	// system on the device.
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

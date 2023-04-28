// Package common contains definitions of fundamental types and functions used
// across multiple file system implementations.
package common

import "math"

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

	// LengthToNumBlocks gives the minimum number of blocks required to hold the
	// given number of bytes.
	LengthToNumBlocks(size uint) uint
}

type BlockDeviceReader interface {
	Read(start LogicalBlock, buffer []byte) error
}

type BlockDeviceWriter interface {
	Write(start LogicalBlock, buffer []byte) error
}

type BlockDeviceReaderWriter interface {
	BlockDeviceReader
	BlockDeviceWriter
}

type BlockDeviceResizer interface {
	Resize(newTotalBlocks uint) error
}

type DiskImage interface {
	BlockDevice
	BlockDeviceReader
}

type WritableDiskImage interface {
	DiskImage
	BlockDeviceWriter
}

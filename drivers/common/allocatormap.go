// Bitmap allocator

package common

import (
	"fmt"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
)

type Allocator struct {
	AllocationBitmap   bitmap.Bitmap
	lastAllocatedIndex uint
	TotalUnits         uint
}

func NewAllocator(totalUnits uint) Allocator {
	return Allocator{
		AllocationBitmap: bitmap.New(int(totalUnits)),
		TotalUnits:       totalUnits,
	}
}

// AllocateBlock allocates the first available block it finds and returns its
// index. If no blocks are available, it returns an error.
func (alloc *Allocator) AllocateBlock() (BlockID, error) {
	for i := uint(0); i < alloc.TotalUnits; i++ {
		if !alloc.AllocationBitmap.Get(int(i)) {
			alloc.AllocationBitmap.Set(int(i), true)
			return BlockID(i), nil
		}
	}

	return 0, disko.NewDriverError(disko.ENOSPC)
}

// FreeBlock frees an allocated block. Trying to free a block that is already
// allocated will return the errno code EALREADY.
func (alloc *Allocator) FreeBlock(block BlockID) error {
	if block >= BlockID(alloc.TotalUnits) {
		msg := fmt.Sprintf(
			"invalid block id: %d not in range [0, %d)",
			block,
			alloc.TotalUnits)
		return disko.NewDriverErrorWithMessage(disko.EINVAL, msg)
	}
	if !alloc.AllocationBitmap.Get(int(block)) {
		msg := fmt.Sprintf("block %d is already free", block)
		return disko.NewDriverErrorWithMessage(disko.EALREADY, msg)
	}

	alloc.AllocationBitmap.Set(int(block), false)
	return nil
}

func (manager *Allocator) findRun(count uint, value bool) (BlockID, error) {
	runSize := uint(0)
	runStart := BlockID(0)

	for i := uint(0); i < manager.TotalUnits; i++ {
		bit := manager.AllocationBitmap.Get(int(i))
		if bit == !value {
			// We hit the opposite value we were looking for, so this is the end
			// of the run. Reset the size to 0 and try again.
			runSize = 0
			continue
		}

		runSize++
		if runSize == 1 {
			// If runSize is 1 then that means it was 0 when we entered the loop,
			// so this is the first unallocated block in our latest attempt at
			// finding a run. This is the beginning of our run.
			runStart = BlockID(i)
		} else if runSize == count {
			// We found the last block we need in the contiguous run.
			return runStart, nil
		}
	}

	// We ran off the end of the bitmap before we reached the necessary count
	// of blocks.
	return BlockID(0), disko.NewDriverError(disko.ENOSPC)
}

// AllocateContiguousBlocks allocates a set of contiguous blocks in a first-fit
// manner.
func (manager *Allocator) AllocateContiguousBlocks(count uint) (BlockID, error) {
	runStart, err := manager.findRun(count, false)
	if err != nil {
		return BlockID(0), err
	}

	for i := uint(0); i < count; i++ {
		manager.AllocationBitmap.Set(int(i+uint(runStart)), true)
	}
	return runStart, nil
}

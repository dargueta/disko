// Bitmap allocator

package common

import (
	"fmt"

	"github.com/boljen/go-bitmap"
	"github.com/dargueta/disko"
)

type UnitID uint32

type Allocator struct {
	AllocationBitmap   bitmap.Bitmap
	lastAllocatedIndex UnitID
	TotalUnits         uint
}

// NewAllocator creates a new allocation bitmap with all bits cleared.
func NewAllocator(totalUnits uint) Allocator {
	return Allocator{
		AllocationBitmap: bitmap.New(int(totalUnits)),
		TotalUnits:       totalUnits,
	}
}

// NewAllocatorFromFreeBitmap creates a new allocator starting from an existing
// bitmap that indicates which units are available for use.
func NewAllocatorFromFreeBitmap(freeMap []byte) Allocator {
	alloc := Allocator{
		AllocationBitmap: bitmap.New(len(freeMap) * 8),
		TotalUnits:       uint(len(freeMap) * 8),
	}

	for i := range freeMap {
		bit := bitmap.Get(freeMap, i)
		alloc.AllocationBitmap.Set(i, !bit)
	}
	return alloc
}

// NewAllocatorFromInUseBitmap creates a new allocator starting from an existing
// bitmap that indicates which units are in use.
func NewAllocatorFromInUseBitmap(inUseMap []byte) Allocator {
	bitmapBuf := make([]byte, len(inUseMap))
	copy(bitmapBuf, inUseMap)
	return Allocator{
		AllocationBitmap: bitmap.Bitmap(bitmapBuf),
		TotalUnits:       uint(len(inUseMap) * 8),
	}
}

// AllocateSingle allocates the first available unit it finds and returns its
// index. If no units are available, it returns an error.
func (alloc *Allocator) AllocateSingle() (UnitID, error) {
	for i := uint(0); i < alloc.TotalUnits; i++ {
		if !alloc.AllocationBitmap.Get(int(i)) {
			alloc.AllocationBitmap.Set(int(i), true)
			alloc.lastAllocatedIndex = UnitID(i)
			return UnitID(i), nil
		}
	}

	return 0, disko.NewDriverError(disko.ENOSPC)
}

// FreeSingle frees an allocated unit. Trying to free a unit that isn't allocated
// will return the errno code EALREADY.
func (alloc *Allocator) FreeSingle(unit UnitID) error {
	if unit >= UnitID(alloc.TotalUnits) {
		msg := fmt.Sprintf(
			"invalid unit id: %d not in range [0, %d)",
			unit,
			alloc.TotalUnits)
		return disko.NewDriverErrorWithMessage(disko.EINVAL, msg)
	}
	if !alloc.AllocationBitmap.Get(int(unit)) {
		msg := fmt.Sprintf("block %d is already free", unit)
		return disko.NewDriverErrorWithMessage(disko.EALREADY, msg)
	}

	alloc.AllocationBitmap.Set(int(unit), false)
	return nil
}

// FindContiguousValues returns the index of the beginning of a block of units
// of length `count` and the same value as `value`.
func (alloc *Allocator) FindContiguousValues(value bool, count uint) (UnitID, error) {
	runSize := uint(0)
	runStart := UnitID(0)

	for i := uint(0); i < alloc.TotalUnits; i++ {
		bit := alloc.AllocationBitmap.Get(int(i))
		if bit == !value {
			// We hit the opposite value we were looking for, so this is the end
			// of the run. Reset the size to 0 and try again.
			runSize = 0
			continue
		}

		runSize++
		if runSize == 1 {
			// If runSize is 1 then that means it was 0 when we entered the loop,
			// so this is the first unallocated unit in our latest attempt at
			// finding a run. This is the beginning of our run.
			runStart = UnitID(i)
		} else if runSize == count {
			// We found the last block we need in the contiguous run.
			return runStart, nil
		}
	}

	// We ran off the end of the bitmap before we reached the necessary count.
	return UnitID(0), disko.NewDriverError(disko.ENOSPC)
}

func (alloc *Allocator) HasContiguousValuesAt(start UnitID, value bool, count uint) bool {
	runSize := uint(0)

	for i := start; i < UnitID(alloc.TotalUnits); i++ {
		if runSize == count {
			// We found the last block we need in the contiguous run.
			return true
		}

		bit := alloc.AllocationBitmap.Get(int(i))
		if bit == !value {
			// We hit the opposite value we were looking for, so this is the end
			// of the run.
			return false
		}
		runSize++
	}

	// We ran off the end of the bitmap before we reached the necessary count.
	return false
}

// AllocateContiguous allocates a set of contiguous units in a first-fit manner.
func (alloc *Allocator) AllocateContiguous(count uint) (UnitID, error) {
	runStart, err := alloc.FindContiguousValues(false, count)
	if err != nil {
		return UnitID(0), err
	}

	for i := uint(0); i < count; i++ {
		alloc.AllocationBitmap.Set(int(i+uint(runStart)), true)
	}
	alloc.lastAllocatedIndex = UnitID(uint(runStart) + count - 1)
	return runStart, nil
}

// FreeContiguous frees a set of contiguous `count` units starting at index
// `start`. If any units in the range are already free, it fails immediately and
// the bitmap is *not* modified.
func (alloc *Allocator) FreeContiguous(start UnitID, count uint) error {
	if !alloc.HasContiguousValuesAt(start, true, count) {
		msg := fmt.Sprintf(
			"tried to free already free blocks: there aren't %d allocated blocks starting at %d",
			count,
			start)
		return disko.NewDriverErrorWithMessage(disko.EINVAL, msg)
	}

	for i := uint(0); i < count; i++ {
		err := alloc.FreeSingle(start + UnitID(i))
		if err != nil {
			return err
		}
	}
	return nil
}

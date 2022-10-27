package driver

import (
	"os"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/file_systems/common"
)

// NopObjectHandle implements the [ObjectHandle] interface, but returns
// [disko.ErrNotImplemented] for all operations. Any non-error return values are
// the corresponding zero value for that type.
type NopObjectHandle struct {
	extObjectHandle
}

// Stat returns an empty [disko.FileStat] struct with all members initialized to
// their zero values.
func (obj NopObjectHandle) Stat() disko.FileStat {
	return disko.FileStat{}
}

// Resize does nothing.
func (obj NopObjectHandle) Resize(newSize uint64) disko.DriverError {
	return disko.ErrNotImplemented
}

// ReadBlocks does nothing.
func (obj NopObjectHandle) ReadBlocks(
	index common.LogicalBlock, buffer []byte,
) disko.DriverError {
	return disko.ErrNotImplemented
}

// WriteBlocks does nothing.
func (obj NopObjectHandle) WriteBlocks(
	index common.LogicalBlock, data []byte,
) disko.DriverError {
	return disko.ErrNotImplemented
}

// ZeroOutBlocks does nothing.
func (obj NopObjectHandle) ZeroOutBlocks(
	startIndex common.LogicalBlock, count uint,
) disko.DriverError {
	return disko.ErrNotImplemented
}

// Unlink does nothing.
func (obj NopObjectHandle) Unlink() disko.DriverError {
	return disko.ErrNotImplemented
}

// Chmod does nothing.
func (obj NopObjectHandle) Chmod(mode os.FileMode) disko.DriverError {
	return disko.ErrNotImplemented
}

// Chown does nothing.
func (obj NopObjectHandle) Chown(uid, gid int) disko.DriverError {
	return disko.ErrNotImplemented
}

// Chtimes does nothing.
func (obj NopObjectHandle) Chtimes(
	createdAt,
	lastAccessed,
	lastModified,
	lastChanged,
	deletedAt time.Time,
) error {
	return disko.ErrNotImplemented
}

// ListDir does nothing, and returns a nil list of names.
func (obj NopObjectHandle) ListDir() ([]string, disko.DriverError) {
	return nil, disko.ErrNotImplemented
}

// Name returns an empty string.
func (obj NopObjectHandle) Name() string {
	return ""
}

// AbsolutePath returns an empty string.
func (obj NopObjectHandle) AbsolutePath() string {
	return ""
}

package driver

import (
	"os"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
	"github.com/dargueta/disko/file_systems/common"
)

// NopObjectHandle implements the [ObjectHandle] interface, but returns an
// error with code [errors.ENOSYS] for all operations. Any non-error return values
// are the corresponding zero value for that type.
type NopObjectHandle struct {
	disko.ObjectHandle
}

// Stat returns an empty [disko.FileStat] struct with all members initialized to
// their zero values.
func (obj NopObjectHandle) Stat() disko.FileStat {
	return disko.FileStat{}
}

// Resize does nothing.
func (obj NopObjectHandle) Resize(newSize uint64) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// ReadBlocks does nothing.
func (obj NopObjectHandle) ReadBlocks(
	index common.LogicalBlock, buffer []byte,
) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// WriteBlocks does nothing.
func (obj NopObjectHandle) WriteBlocks(
	index common.LogicalBlock, data []byte,
) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// ZeroOutBlocks does nothing.
func (obj NopObjectHandle) ZeroOutBlocks(
	startIndex common.LogicalBlock, count uint,
) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// Unlink does nothing.
func (obj NopObjectHandle) Unlink() errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// Chmod does nothing.
func (obj NopObjectHandle) Chmod(mode os.FileMode) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// Chown does nothing.
func (obj NopObjectHandle) Chown(uid, gid int) errors.DriverError {
	return errors.NewDriverError(errors.ENOSYS)
}

// Chtimes does nothing.
func (obj NopObjectHandle) Chtimes(
	createdAt,
	lastAccessed,
	lastModified,
	lastChanged,
	deletedAt time.Time,
) error {
	return errors.NewDriverError(errors.ENOSYS)
}

// ListDir does nothing, and returns a nil list of names.
func (obj NopObjectHandle) ListDir() ([]string, errors.DriverError) {
	return nil, errors.NewDriverError(errors.ENOSYS)
}

// Name returns an empty string.
func (obj NopObjectHandle) Name() string {
	return ""
}

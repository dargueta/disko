package fat8

import (
	"bytes"
	"fmt"
	"os"

	"github.com/dargueta/disko"
)

type FAT8Driver struct {
	disko.ReadingDriver
	disko.WritingDriver
	disko.FormattingDriver
	sectorsPerTrack      int
	totalTracks          int
	infoSectorIndex      int
	image                *os.File
	stat                 disko.FSStat
	isMounted            bool
	defaultFileAttrFlags uint8
}

////////////////////////////////////////////////////////////////////////////////
// General utility functions

func (driver *FAT8Driver) readSectors(track, sector, count uint) ([]byte, error) {
	return nil, disko.NewDriverError(disko.ENOSYS)
}

func (driver *FAT8Driver) writeSectors(track, startSector uint, data []byte) error {
	return disko.NewDriverError(disko.ENOSYS)
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *FAT8Driver) Mount(flags disko.MountFlags) error {
	// Ignore attempts to mount the drive multiple times.
	if driver.isMounted {
		return disko.NewDriverError(disko.EALREADY)
	}

	offset, err := driver.image.Seek(0, 2)
	if err != nil {
		return err
	}

	if offset == 256256 {
		driver.sectorsPerTrack = 26
		driver.totalTracks = 77
		driver.infoSectorIndex = 19
	} else if offset == 92160 {
		driver.sectorsPerTrack = 18
		driver.totalTracks = 40
		driver.infoSectorIndex = 12
	} else {
		message := fmt.Sprintf(
			"invalid disk image size; expected 256256 or 92160, got %d",
			offset)
		return disko.NewDriverErrorWithMessage(disko.EMEDIUMTYPE, message)
	}

	// TODO (dargueta): Extract FAT

	return nil
}

func (driver *FAT8Driver) GetFSInfo() disko.FSStat {
	return driver.stat
}

////////////////////////////////////////////////////////////////////////////////
// Implementing FormattingDriver interface

// Format creates a new empty disk image using the given disk information.
//
// This driver only requires the TotalBlocks field to be set in `information`. It
// must either be 2002 for a floppy image, or 720 for a minifloppy image.
func (driver *FAT8Driver) Format(information disko.FSStat) error {
	if information.TotalBlocks == 2002 {
		driver.sectorsPerTrack = 26
		driver.totalTracks = 77
		driver.infoSectorIndex = 19
	} else if information.TotalBlocks == 720 {
		driver.sectorsPerTrack = 18
		driver.totalTracks = 40
		driver.infoSectorIndex = 12
	} else {
		return fmt.Errorf(
			"invalid format configuration: TotalBlocks must be 2002 or 720, got %d",
			information.TotalBlocks)
	}

	// Create a blank image filled with null bytes
	driver.image.Seek(0, 0)
	driver.image.Truncate(0)
	driver.image.Write(bytes.Repeat([]byte{0}, int(128*driver.sectorsPerTrack*driver.totalTracks)))
	trackSize := 128 * driver.sectorsPerTrack

	// The directory track is in the middle of the disk.
	directoryTrackNumber := driver.totalTracks / 2
	directoryTrackStart := trackSize * directoryTrackNumber

	// There are two clusters per track, so the size of the FAT is one byte per
	// cluster plus some padding bytes to get to a multiple of the sector size.
	fatSizeInBytes := int(driver.totalTracks * 2)
	if fatSizeInBytes%128 != 0 {
		fatSizeInBytes += 128 - (fatSizeInBytes % 128)
	}

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	fat := bytes.Repeat([]byte{0xff}, fatSizeInBytes)
	fat[directoryTrackNumber*2] = 0xfe
	fat[directoryTrackNumber*2+1] = 0xfe

	// Write the FATs
	fatStart := directoryTrackStart + trackSize - (3 * fatSizeInBytes)
	driver.image.Seek(int64(fatStart), 0)
	_, err := driver.image.Write(bytes.Repeat(fat, 3))

	if err != nil {
		return err
	}

	// We reserve one track for the directory, so the total number of available
	// blocks is one track's worth of blocks fewer.
	availableBlocks := information.TotalBlocks - uint64(driver.sectorsPerTrack)

	// The maximum number of files is:
	// (SectorsPerTrack - 1 - (FatSizeInSectors * 3)) * DirentsPerSector
	// A directory entry is 16 bytes, so there are 8 dirents per sector
	direntSectors := driver.sectorsPerTrack - 1 - ((fatSizeInBytes * 3) / 128)
	totalDirents := direntSectors * 8

	driver.stat = disko.FSStat{
		BlockSize:       128,
		TotalBlocks:     information.TotalBlocks,
		BlocksFree:      availableBlocks,
		BlocksAvailable: availableBlocks,
		Files:           0,
		FilesFree:       uint64(totalDirents),
		// This isn't completely accurate; names are 6.3 format so the longest
		// bare name is six characters, plus an extra three for the extension,
		// plus one more for the ".". Problem is, "ABCDEFGHI" is interpreted as
		// "ABCDEF.GHI" because of the implicit period.
		MaxNameLength: 10,
	}

	return nil
}

/*
// OpeningDriver is the interface for drivers implementing the POSIX open(3) function.
//
// Drivers need not implement all functionality for valid flags. For example, read-only
// drivers must return an error if called with the os.O_CREATE flag.
type OpeningDriver interface {
	// OpenFile is equivalent to os.OpenFile.
	OpenFile(path string, flag int, perm os.FileMode) (*os.File, error)
}

// ReadingDriver is the interface for drivers supporting read operations.
type ReadingDriver interface {
	Readlink(path string) (string, error)
	SameFile(fi1, fi2 os.FileInfo) bool
	Open(path string) (*os.File, error)
	ReadDir(path string) ([]os.FileInfo, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use a
	// reasonable default value. For most of these 0 is fine, but for compatibility
	// drivers should use 1 for `Nlink` and 0o777 for `Mode`.
	Stat(path string) (disko.FileStat, error)
	// Lstat returns the same information as Stat but follows symbolic links. On file
	// systems that don't support symbolic links, the behavior is exactly the same as
	// Stat.
	Lstat(path string) (disko.FileStat, error)
}

// WritingDriver is the interface for drivers supporting write operations.
type WritingDriver interface {
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Repath(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (*os.File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
}
*/

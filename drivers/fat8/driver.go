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
	// image is a file object for the file the disk image is for.
	image                *os.File
	// sectorsPerTrack defines the number of sectors in a single track for the
	// current disk geometry.
	sectorsPerTrack      uint
	// totalTracks gives the number of tracks for the current disk geometry.
	totalTracks          uint
	// infoSectorIndex is the zero-based index of the "information sector" of a
	// FAT8 image.
	//
	// To my knowledge the FAT8 standard only defines the first byte of this
	// sector, which is the default attribute byte to use for new files.
	infoSectorIndex      uint
	defaultFileAttrFlags uint8
	stat                 disko.FSStat
	// freeClusters is an array of the indexes of all unallocated clusters. This
	// will never be more than 189 entries long.
	freeClusters         []uint8
	// fat is the FAT as represented on the disk. It's always a multiple of 128
	// in length, but only the first totalTracks*2 entries are valid. Anything
	// beyond that must not be modified.
	fat                  []uint8
	// isMounted indicates if the drive is currently mounted.
	isMounted            bool
}

////////////////////////////////////////////////////////////////////////////////
// General utility functions

func (driver *FAT8Driver) trackAndSectorToFileOffset(track, sector uint) (int64, error) {
	if track >= driver.totalTracks {
		return -1,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid track number: %d not in [0, %d)",
					track,
					driver.totalTracks,
				),
			)
	}
	if sector >= driver.sectorsPerTrack {
		return -1,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid sector number: %d not in [0, %d)",
					sector,
					driver.sectorsPerTrack,
				),
			)
	}

	absoluteSector := (track * driver.sectorsPerTrack) + sector
	return int64(absoluteSector * 128), nil
}

func (driver *FAT8Driver) readSectors(track, sector, count uint) ([]byte, error) {
	offset, err := driver.trackAndSectorToFileOffset(track, sector)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 128*count)

	driver.image.ReadAt(buffer, offset)
	_, err = driver.image.Read(buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func (driver *FAT8Driver) writeSectors(track, sector uint, data []byte) error {
	offset, err := driver.trackAndSectorToFileOffset(track, sector)
	if err != nil {
		return err
	}

	imageSize := driver.stat.TotalBlocks * 128
	if uint64(offset)+uint64(len(data)) >= imageSize {
		return disko.NewDriverErrorWithMessage(
			disko.EINVAL,
			fmt.Sprintf(
				"can't write %d bytes at track %d, sector %d: extends past end of image",
				len(data),
				track,
				sector,
			),
		)
	}

	_, err = driver.image.WriteAt(data, offset)
	return err
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

	// Each track is two clusters, so the number of entries in the FAT is two
	// times the number of tracks. Each entry is one byte, so if we do a ceiling
	// division of the number of bytes in a FAT by 128, we'll get the FAT size
	// in sectors.
	//
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := uint((totalClusters + (-totalClusters % 128)) / 128)
	directoryTrack := driver.totalTracks / 2

	// There are three copies of the FAT at the end of the directory track. Read
	// each one and ensure they're all identical. If they're not, that's likely
	// an indicator of disk corruption.
	firstFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*3),
		fatSizeInSectors)
	if err != nil {
		return err
	}

	secondFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*3),
		fatSizeInSectors)
	if err != nil {
		return err
	}

	thirdFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*3),
		fatSizeInSectors)
	if err != nil {
		return err
	}

	if !bytes.Equal(firstFAT, secondFAT) {
		return disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 2")
	} else if !bytes.Equal(firstFAT, thirdFAT) {
		return disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 3")
	}

	// All FATs are identical, so we only need to store the first one.
	driver.fat = firstFAT

	// Build a list of all currently free clusters.
	for i, clusterNumber := range firstFAT {
		if clusterNumber == 0xff {
			driver.freeClusters = append(driver.freeClusters, uint8(i))
		}
	}

	// Get the info sector. The first byte of the info sector tells us what the
	// default attributes should be for new files; the rest is undefined.
	infoSector, err := driver.readSectors(directoryTrack, driver.infoSectorIndex, 1)
	if err != nil {
		return err
	}
	driver.defaultFileAttrFlags = infoSector[0]

	return nil
}

// TODO (dargueta): Unmount()

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
	driver.image.Write(
		bytes.Repeat([]byte{0}, int(128*driver.sectorsPerTrack*driver.totalTracks)))

	// There are two clusters per track, so the size of the FAT is one byte per
	// cluster plus some padding bytes to get to a multiple of the sector size.
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := (totalClusters + (-totalClusters % -128)) / 128

	// The directory track is in the middle of the disk.
	directoryTrackNumber := driver.totalTracks / 2

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	fat := bytes.Repeat([]byte{0xff}, fatSizeInSectors*128)
	fat[directoryTrackNumber*2] = 0xfe
	fat[directoryTrackNumber*2+1] = 0xfe

	allFATs := bytes.Repeat(fat, 3)

	// Write the FATs
	fatStart := driver.sectorsPerTrack - uint(fatSizeInSectors*3)
	err := driver.writeSectors(directoryTrackNumber, fatStart, allFATs)

	if err != nil {
		return err
	}

	// We reserve one track for the directory, so the total number of available
	// blocks is one track's worth of blocks fewer.
	availableBlocks := information.TotalBlocks - uint64(driver.sectorsPerTrack)

	// The maximum number of files is:
	// (SectorsPerTrack - 1 - (FatSizeInSectors * 3)) * DirentsPerSector
	//   * We subtract one for the information sector.
	//   * A directory entry is 16 bytes, so there are 8 dirents per sector.
	direntSectors := driver.sectorsPerTrack - 1 - uint(fatSizeInSectors*3)
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

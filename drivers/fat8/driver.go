package fat8

import (
	"fmt"
	"os"

	"github.com/dargueta/disko"
)

type DirectoryEntry struct {
	disko.DirectoryEntry
	name                       string
	clusters                   []uint8
	index                      uint
	IsBinary                   bool
	IsEBCDIC                   bool
	IsWriteProtected           bool
	ReadAfterWriteEnabled      bool
	UnusedSectorsInLastCluster uint
}

type Driver struct {
	disko.ReadingDriver
	disko.WritingDriver
	disko.FormattingDriver
	// image is a file object for the file the disk image is for.
	image *os.File
	// sectorsPerTrack defines the number of sectors in a single track for the
	// current disk geometry.
	sectorsPerTrack uint
	// totalTracks gives the number of tracks for the current disk geometry.
	totalTracks uint
	// bytesPerCluster gives the number of bytes in a single cluster.
	bytesPerCluster uint
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
	freeClusters []uint8
	// fat is the FAT as represented on the disk. It's always a multiple of 128
	// in length, but only the first totalTracks*2 entries are valid. Anything
	// beyond that must not be modified.
	fat []uint8
	// isMounted indicates if the drive is currently mounted.
	isMounted bool
	dirents   map[string]DirectoryEntry
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *Driver) Mount(flags disko.MountFlags) error {
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

	driver.bytesPerCluster = driver.sectorsPerTrack * 64

	// All FATs are identical, so we only need to store the first one.
	fat, err := driver.readFATs()
	if err != nil {
		return err
	}
	driver.fat = fat

	// Build a list of all currently free clusters.
	for i, clusterNumber := range fat {
		if clusterNumber == 0xff {
			driver.freeClusters = append(driver.freeClusters, uint8(i+1))
		}
	}

	// The directory track is always the middle one in the disk.
	directoryTrack := driver.totalTracks / 2

	// Get the info sector, which is always at a fixed location in the directory
	// track. The first byte of the info sector tells us what the default
	// attributes should be for new files; the rest is not defined by the standard.
	infoSector, err := driver.readSectors(directoryTrack, driver.infoSectorIndex, 1)
	if err != nil {
		return err
	}
	driver.defaultFileAttrFlags = infoSector[0]
	return nil
}

// TODO (dargueta): Unmount()

func (driver *Driver) GetFSInfo() disko.FSStat {
	return driver.stat
}

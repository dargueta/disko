package fat8

import (
	"fmt"
	"os"

	"github.com/dargueta/disko"
)

type LogicalBlock uint
type PhysicalBlock uint
type LogicalCluster uint
type PhysicalCluster uint

type DirectoryEntry struct {
	disko.DirectoryEntry
	name                       string
	clusters                   []PhysicalCluster
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
	infoSectorStart      PhysicalBlock
	directoryTrackStart  PhysicalBlock
	fatsStart            PhysicalBlock
	fatSizeInSectors     uint
	totalClusters        uint
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

func NewDriverFromFile(stream *os.File) Driver {
	return Driver{image: stream}
}

func (driver *Driver) defineGeometry(totalBlocks uint) error {
	if totalBlocks == 2002 {
		driver.sectorsPerTrack = 26
		driver.totalTracks = 77
	} else if totalBlocks == 720 {
		driver.sectorsPerTrack = 18
		driver.totalTracks = 40
	} else {
		message := fmt.Sprintf(
			"invalid disk image size; expected 2002 or 720, got %d",
			totalBlocks,
		)
		return disko.NewDriverErrorWithMessage(disko.EMEDIUMTYPE, message)
	}

	// There are two clusters per track, so the size of the FAT is one byte per
	// cluster plus some padding bytes to get to a multiple of the sector size.
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := uint((totalClusters + (-totalClusters % 128)) / 128)

	directoryTrackStart := driver.totalTracks / 2 * driver.sectorsPerTrack
	fatsStart := uint(driver.directoryTrackStart) + driver.sectorsPerTrack - fatSizeInSectors

	driver.directoryTrackStart = PhysicalBlock(directoryTrackStart)
	driver.fatSizeInSectors = fatSizeInSectors
	driver.fatsStart = PhysicalBlock(fatsStart)
	driver.infoSectorStart = driver.fatsStart - 1
	driver.totalClusters = uint(totalClusters)
	driver.bytesPerCluster = (driver.sectorsPerTrack / 2) * 128
	return nil
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

	driver.defineGeometry(uint(offset))

	// All FATs are identical, so we only need to store the first one.
	fat, err := driver.GetFAT()
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

	// Get the info sector, which always immediately precedes the FATs. The first
	// byte of the info sector tells us what the default attributes should be for
	// new files; the rest is not defined by the standard.
	infoSector, err := driver.ReadDiskBlocks(driver.infoSectorStart, 1)
	if err != nil {
		return err
	}
	driver.defaultFileAttrFlags = infoSector[0]
	return nil
}

func (driver *Driver) Unmount() error {
	return driver.writeFAT()
}

func (driver *Driver) GetFSInfo() disko.FSStat {
	return driver.stat
}

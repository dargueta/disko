package fat8

import (
	"bytes"
	"fmt"

	"github.com/dargueta/disko"
)

////////////////////////////////////////////////////////////////////////////////
// Implementing FormattingDriver interface

// Format creates a new empty disk image using the given disk information.
//
// This driver only requires the TotalBlocks field to be set in `information`.
// It must either be 2002 for a floppy image, or 720 for a minifloppy image.
func (driver *Driver) Format(information disko.FSStat) error {
	if driver.isMounted {
		return disko.NewDriverErrorWithMessage(
			disko.EBUSY,
			"image must be unmounted before it can be formatted")
	}

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
	fileSize := 128 * information.TotalBlocks
	driver.image.Seek(0, 0)
	driver.image.Truncate(int64(fileSize))
	driver.image.Write(bytes.Repeat([]byte{0}, int(fileSize)))

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

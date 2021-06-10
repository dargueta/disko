package fat8

import (
	"bytes"

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

	err := driver.defineGeometry(uint(information.TotalBlocks))
	if err != nil {
		return err
	}

	// Create a blank image filled with null bytes
	fileSize := 128 * information.TotalBlocks
	driver.image.Seek(0, 0)
	driver.image.Truncate(int64(fileSize))
	driver.image.Write(bytes.Repeat([]byte{0}, int(fileSize)))

	// According to the documentation, a newly formatted image must have the
	// directory entries filled with 0xFF.
	sectorFill := bytes.Repeat([]byte{0xff}, 128)
	for i := driver.directoryTrackStart; i < driver.infoSectorStart; i++ {
		driver.WriteDiskBlocks(i, sectorFill)
	}

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	directoryCluster := uint(driver.directoryTrackStart) / driver.sectorsPerTrack
	fat := bytes.Repeat([]byte{0xff}, int(driver.fatSizeInSectors)*128)
	fat[directoryCluster*2] = 0xfe
	fat[directoryCluster*2+1] = 0xfe

	// Write the FATs
	allFATs := bytes.Repeat(fat, 3)
	err = driver.WriteDiskBlocks(driver.fatsStart, allFATs)

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
	direntSectors := driver.sectorsPerTrack - 1 - (driver.fatSizeInSectors * 3)
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

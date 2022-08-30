package fat8

import (
	"bytes"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
)

////////////////////////////////////////////////////////////////////////////////
// Implementing FormattingDriver interface

// Format creates a new empty disk image using the given disk information.
//
// This driver only requires the TotalBlocks field to be set in `information`.
// It must either be 1898 for a floppy image, or 640 for a minifloppy image.
// 2002 is accepted as a synonym for 1898.
func (driver *Driver) Format(information disko.FSStat) error {
	if driver.isMounted {
		return errors.NewWithMessage(
			errors.EBUSY,
			"image must be unmounted before it can be formatted")
	}

	geo, err := GetGeometry(uint(information.TotalBlocks()))
	if err != nil {
		return err
	}

	// We reserve one track for the directory, so the total number of available
	// blocks is one track's worth of blocks fewer.
	availableBlocks := uint64((geo.TotalTracks - 1) * geo.SectorsPerTrack)

	// The maximum number of files is:
	// (SectorsPerTrack - 1 - (FatSizeInSectors * 3)) * DirentsPerSector
	//   * We subtract one for the information sector.
	//   * A directory entry is 16 bytes, so there are 8 dirents per sector.
	direntSectors := geo.SectorsPerTrack - 1 - (geo.SectorsPerFAT * 3)
	totalDirents := direntSectors * 8

	driver.geometry = geo
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

	// Create a blank image filled with null bytes
	fileSize := 128 * geo.TrueTotalTracks * geo.SectorsPerTrack
	driver.image.Truncate(int64(fileSize))

	// According to the documentation, a newly formatted image must have the
	// directory entries filled with 0xFF.
	sectorFill := bytes.Repeat([]byte{0xff}, 128)
	for i := geo.DirectoryTrackStart; i < geo.InfoSectorStart; i++ {
		err := driver.WriteDiskBlocks(i, sectorFill)
		if err != nil {
			return err
		}
	}

	// Write nulls to the info sector
	err = driver.WriteDiskBlocks(geo.InfoSectorStart, bytes.Repeat([]byte{0}, 128))
	if err != nil {
		return err
	}

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	directoryCluster := uint(geo.DirectoryTrackStart) / geo.SectorsPerTrack
	fat := bytes.Repeat([]byte{0xff}, int(geo.SectorsPerFAT)*128)
	fat[directoryCluster*2] = 0xfe
	fat[directoryCluster*2+1] = 0xfe

	// Write the FATs
	allFATs := bytes.Repeat(fat, 3)
	return driver.WriteDiskBlocks(geo.FATsStart, allFATs)
}

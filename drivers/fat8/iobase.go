package fat8

import (
	"bytes"
	"fmt"

	"github.com/dargueta/disko"
)

func (driver *Driver) trackAndSectorToFileOffset(track, sector uint) (int64, error) {
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

func (driver *Driver) readSectors(track, sector, count uint) ([]byte, error) {
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

func (driver *Driver) ReadCluster(clusterID uint) ([]byte, error) {
	if (clusterID < 1) || (clusterID >= 0xc0) {
		return nil,
			fmt.Errorf("bad cluster number: %#x not in [1, 0xc0)", clusterID)
	}
	track := (clusterID - 1) / 2
	trackCluster := (clusterID - 1) % 2
	startingSector := trackCluster * driver.sectorsPerTrack / 2
	return driver.readSectors(track, startingSector, driver.sectorsPerTrack/2)
}

func (driver *Driver) writeSectors(track, sector uint, data []byte) error {
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

// WriteCluster writes bytes to the given cluster. `data` must be exactly the
// size of a cluster.
func (driver *Driver) WriteCluster(clusterID uint, data []byte) error {
	if (clusterID < 1) || (clusterID >= 0xc0) {
		return fmt.Errorf("bad cluster number: %#x not in [1, 0xc0)", clusterID)
	}
	if len(data) != int(driver.bytesPerCluster) {
		return fmt.Errorf(
			"data to write is the wrong size: expected %d, got %d",
			driver.bytesPerCluster,
			len(data))
	}

	track := (clusterID - 1) / 2
	trackCluster := (clusterID - 1) % 2
	startingSector := trackCluster * driver.sectorsPerTrack / 2
	return driver.writeSectors(track, startingSector, data)
}

func (driver *Driver) readFATs() ([]byte, error) {
	// The directory track is always the middle one in the disk.
	directoryTrack := driver.totalTracks / 2

	// Each track is two clusters, so the number of entries in the FAT is two
	// times the number of tracks. Each entry is one byte, so if we do a ceiling
	// division of the number of bytes in a FAT by 128, we'll get the FAT size
	// in sectors.
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := uint((totalClusters + (-totalClusters % 128)) / 128)

	// There are three copies of the FAT at the end of the directory track. Read
	// each one and ensure they're all identical. If they're not, that's likely
	// an indicator of disk corruption.
	firstFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*3),
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	secondFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*2),
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	thirdFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-fatSizeInSectors,
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(firstFAT, secondFAT) {
		return nil, disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 2")
	} else if !bytes.Equal(firstFAT, thirdFAT) {
		return nil, disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 3")
	}

	return firstFAT, nil
}

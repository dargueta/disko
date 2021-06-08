package fat8

import (
	"bytes"
	"fmt"

	"github.com/dargueta/disko"
)

// BLOCK-LEVEL ACCESS ==========================================================

func (driver *Driver) TrackAndSectorToBlock(track uint, sector PhysicalBlock) (PhysicalBlock, error) {
	if track >= driver.totalTracks {
		return 0,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid track number: %d not in [0, %d)",
					track,
					driver.totalTracks,
				),
			)
	}
	if uint(sector) >= driver.sectorsPerTrack {
		return 0,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid sector number: %d not in [0, %d)",
					sector,
					driver.sectorsPerTrack,
				),
			)
	}

	return PhysicalBlock(track*driver.sectorsPerTrack) + sector, nil
}

func (driver *Driver) ReadDiskBlocks(start PhysicalBlock, count uint) ([]byte, error) {
	if (uint(start) + count) >= uint(driver.stat.TotalBlocks) {
		return nil, fmt.Errorf(
			"refusing to read past end of image: %d blocks at %d exceeds limit of %d",
			start,
			count,
			driver.stat.TotalBlocks,
		)
	}

	buffer := make([]byte, 128*count)
	_, err := driver.image.ReadAt(buffer, int64(start)*128)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func (driver *Driver) WriteDiskBlocks(start PhysicalBlock, data []byte) error {
	if len(data)%128 != 0 {
		return disko.NewDriverErrorWithMessage(
			disko.EIO,
			fmt.Sprintf("data buffer must be a multiple of 128 bytes, got %d", len(data)),
		)
	}

	numBlocksToWrite := uint64(len(data) / 128)
	if uint64(start)+numBlocksToWrite >= driver.stat.TotalBlocks {
		return disko.NewDriverErrorWithMessage(
			disko.EIO,
			fmt.Sprintf(
				"refusing to write past end of image: %d blocks at %d exceeds limit of %d",
				numBlocksToWrite,
				start,
				driver.stat.TotalBlocks,
			),
		)
	}

	_, err := driver.image.WriteAt(data, int64(start)*128)
	return err
}

// CLUSTER-LEVEL ACCESS ========================================================

// IsValidCluster returns a boolean indicating whether the given cluster number
// is valid for this disk image.
func (driver *Driver) IsValidCluster(clusterID PhysicalCluster) bool {
	return (clusterID >= 1) && (uint(clusterID) <= driver.totalTracks*2)
}

func MakeInvalidClusterError(cluster PhysicalCluster, totalTracks uint) error {
	return fmt.Errorf(
		"bad cluster number: %#02x not in range [1, %#02x]",
		cluster,
		totalTracks*2)
}

// ReadAbsoluteCluster reads the given cluster from the disk. `clusterID` must
// be valid, as determined by IsValidCluster().
func (driver *Driver) ReadAbsoluteCluster(clusterID PhysicalCluster) ([]byte, error) {
	if !driver.IsValidCluster(clusterID) {
		return nil, MakeInvalidClusterError(clusterID, driver.totalTracks)
	}
	sectorsPerCluster := driver.sectorsPerTrack / 2
	block := uint(clusterID) * sectorsPerCluster
	return driver.ReadDiskBlocks(PhysicalBlock(block), sectorsPerCluster)
}

// WriteAbsoluteCluster writes bytes to the given cluster. `data` must be exactly
// the size of a cluster.
func (driver *Driver) WriteAbsoluteCluster(clusterID PhysicalCluster, data []byte) error {
	if !driver.IsValidCluster(clusterID) {
		return MakeInvalidClusterError(clusterID, driver.totalTracks)
	}
	if len(data) != int(driver.bytesPerCluster) {
		return fmt.Errorf(
			"data to write is the wrong size: expected %d, got %d",
			driver.bytesPerCluster,
			len(data))
	}

	physicalBlock := uint(clusterID) * driver.sectorsPerTrack
	return driver.WriteDiskBlocks(PhysicalBlock(physicalBlock), data)
}

func (driver *Driver) GetFAT() ([]byte, error) {
	// There are three copies of the FAT at the end of the directory track. Read
	// each one and ensure they're all identical. If they're not, that's likely
	// an indicator of disk corruption.
	firstFAT, err := driver.ReadDiskBlocks(
		driver.fatsStart, driver.fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	secondFAT, err := driver.ReadDiskBlocks(
		driver.fatsStart+PhysicalBlock(driver.fatSizeInSectors),
		driver.fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	thirdFAT, err := driver.ReadDiskBlocks(
		driver.fatsStart+PhysicalBlock(2*driver.fatSizeInSectors),
		driver.fatSizeInSectors)
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

// FILE-LEVEL ACCESS ===========================================================

func (driver *Driver) ReadFileCluster(dirent *DirectoryEntry, cluster LogicalCluster) ([]byte, error) {
	if int(cluster) >= len(dirent.clusters) {
		return nil, fmt.Errorf(
			"cluster index out of bounds: %d not in [0, %d)",
			cluster,
			len(dirent.clusters),
		)
	}

	physicalCluster := dirent.clusters[cluster]
	sectorsPerCluster := driver.sectorsPerTrack / 2
	physicalBlock := PhysicalBlock(uint(physicalCluster) * sectorsPerCluster)
	return driver.ReadDiskBlocks(physicalBlock, sectorsPerCluster)
}

func (driver *Driver) WriteFileCluster(dirent *DirectoryEntry, cluster LogicalCluster, data []byte) error {
	if int(cluster) >= len(dirent.clusters) {
		return fmt.Errorf(
			"cluster index out of bounds: %d not in [0, %d)",
			cluster,
			len(dirent.clusters),
		)
	}

	physicalCluster := dirent.clusters[cluster]
	return driver.WriteAbsoluteCluster(physicalCluster, data)
}

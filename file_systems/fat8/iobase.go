package fat8

import (
	"bytes"
	"fmt"

	"github.com/dargueta/disko/errors"
)

// BLOCK-LEVEL ACCESS ==========================================================

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
		return errors.ErrIOFailed.WithMessage(
			fmt.Sprintf("data buffer must be a multiple of 128 bytes, got %d", len(data)),
		)
	}

	numBlocksToWrite := uint64(len(data) / 128)
	if uint64(start)+numBlocksToWrite > driver.stat.TotalBlocks {
		return errors.ErrIOFailed.WithMessage(
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
	return (clusterID >= 1) && (uint(clusterID) <= driver.geometry.TotalClusters)
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
		return nil, MakeInvalidClusterError(clusterID, driver.geometry.TotalTracks)
	}
	block := uint(clusterID-1) * driver.geometry.SectorsPerCluster
	return driver.ReadDiskBlocks(PhysicalBlock(block), driver.geometry.SectorsPerCluster)
}

// WriteAbsoluteCluster writes bytes to the given cluster. `data` must be exactly
// the size of a cluster.
func (driver *Driver) WriteAbsoluteCluster(clusterID PhysicalCluster, data []byte) error {
	if !driver.IsValidCluster(clusterID) {
		return MakeInvalidClusterError(clusterID, driver.geometry.TotalTracks)
	}
	if len(data) != int(driver.geometry.BytesPerCluster) {
		return fmt.Errorf(
			"data to write is the wrong size: expected %d, got %d",
			driver.geometry.BytesPerCluster,
			len(data))
	}

	physicalBlock := uint(clusterID-1) * driver.geometry.SectorsPerTrack
	return driver.WriteDiskBlocks(PhysicalBlock(physicalBlock), data)
}

func (driver *Driver) GetFAT() ([]byte, error) {
	// There are three copies of the FAT at the end of the directory track. Read
	// each one and ensure they're all identical. If they're not, that's likely
	// an indicator of disk corruption.
	firstFAT, err := driver.ReadDiskBlocks(
		driver.geometry.FATsStart, driver.geometry.SectorsPerFAT)
	if err != nil {
		return nil, err
	}

	secondFAT, err := driver.ReadDiskBlocks(
		driver.geometry.FATsStart+PhysicalBlock(driver.geometry.SectorsPerFAT),
		driver.geometry.SectorsPerFAT)
	if err != nil {
		return nil, err
	}

	thirdFAT, err := driver.ReadDiskBlocks(
		driver.geometry.FATsStart+PhysicalBlock(2*driver.geometry.SectorsPerFAT),
		driver.geometry.SectorsPerFAT)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(firstFAT, secondFAT) {
		return nil, errors.ErrFileSystemCorrupted.WithMessage(
			"disk corruption detected: FAT copy 1 differs from FAT copy 2")
	} else if !bytes.Equal(firstFAT, thirdFAT) {
		return nil, errors.ErrFileSystemCorrupted.WithMessage(
			"disk corruption detected: FAT copy 1 differs from FAT copy 3")
	}

	return firstFAT, nil
}

func (driver *Driver) writeFAT() error {
	return driver.WriteDiskBlocks(driver.geometry.FATsStart, bytes.Repeat(driver.fat, 3))
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
	physicalBlock := PhysicalBlock(uint(physicalCluster) * driver.geometry.SectorsPerCluster)
	return driver.ReadDiskBlocks(physicalBlock, driver.geometry.SectorsPerCluster)
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

package fat

import (
	"fmt"
	"io"
	"syscall"

	disko "github.com/dargueta/disko"
)

// This file defines the driver interface and delegates to the underlying version-specific
// drivers.

type ClusterID uint32
type SectorID uint32

type FATDriverCommon interface {
	GetBootSector() *FATBootSector
	GetClusterAtIndex(index uint) (ClusterID, error)
	SetClusterAtIndex(index uint, cluster ClusterID) error
	GetNextClusterInChain(cluster ClusterID) (ClusterID, error)
	IsValidCluster(cluster ClusterID) bool
	IsEndOfChain(cluster ClusterID) bool
}

type FATDriver struct {
	fs       FATDriverCommon
	diskFile interface{}
}

func (drv *FATDriver) getFirstSectorOfCluster(cluster ClusterID) (SectorID, error) {
	bootSector := drv.fs.GetBootSector()
	return bootSector.FirstDataSector + SectorID(
		uint32(bootSector.SectorsPerCluster)*uint32(cluster)), nil
}

func (drv *FATDriver) readAbsoluteSectors(sector SectorID, numSectors uint) ([]byte, error) {
	bootSector := drv.fs.GetBootSector()

	buffer := make([]byte, bootSector.BytesPerSector)
	diskFile := drv.diskFile.(io.ReaderAt)

	nRead, err := diskFile.ReadAt(buffer, int64(bootSector.BytesPerSector)*int64(sector)*int64(numSectors))

	if err != nil {
		return buffer, err
	} else if nRead < int(bootSector.BytesPerSector) {
		return nil, fmt.Errorf(
			"Unexpected short read. Wanted %d bytes, got %d.", bootSector.BytesPerSector, nRead)
	}

	return buffer, nil
}

// readCluster returns the bytes of the `index`th cluster on the file system.
func (drv *FATDriver) readCluster(cluster ClusterID, index uint) ([]byte, error) {
	sectorID, err := drv.getFirstSectorOfCluster(cluster)
	if err != nil {
		return nil, err
	}

	bootSector := drv.fs.GetBootSector()
	return drv.readAbsoluteSectors(sectorID, uint(bootSector.SectorsPerCluster))
}

// readSectorInCluster returns the bytes of the `index`th sector of the given cluster.
// `index` starts from 0. On error, the byte slice will be nil and the second return value
// is an error object detailing what went wrong.
func (drv *FATDriver) readSectorsInCluster(cluster ClusterID, index uint, numSectors uint) ([]byte, error) {
	firstSector, err := drv.getFirstSectorOfCluster(cluster)
	if err != nil {
		return nil, err
	}

	bootSector := drv.fs.GetBootSector()
	if (index + numSectors) >= uint(bootSector.SectorsPerCluster) {
		return nil, disko.NewDriverErrorWithMessage(
			syscall.ERANGE,
			fmt.Sprintf(
				"cannot read %d sectors from index %d: read would exceed cluster size",
				index,
				numSectors))
	}

	absoluteSector := uint(firstSector) + index
	return drv.readAbsoluteSectors(SectorID(absoluteSector), numSectors)
}

// listClusters returns a list of every cluster in the chain beginning at chainStart.
//
// The returned list will always have chainStart as its first member, unless chainStart
// is an EOF marker (e.g. 0xFFF on FAT12 systems). In this case, the list is empty.
func (drv *FATDriver) listClusters(chainStart ClusterID) ([]ClusterID, error) {
	if !drv.fs.IsValidCluster(chainStart) {
		return nil, disko.NewDriverErrorWithMessage(
			syscall.EINVAL,
			fmt.Sprintf("invalid cluster 0x%x cannot start a cluster chain", chainStart))
	}

	chain := []ClusterID{}
	currentCluster := chainStart
	i := 0

	for !drv.fs.IsEndOfChain(currentCluster) {
		chain = append(chain, currentCluster)

		nextCluster, err := drv.fs.GetClusterAtIndex(uint(currentCluster))
		if err != nil {
			return nil, err
		}

		if !drv.fs.IsValidCluster(nextCluster) {
			// Hit an invalid cluster. This is not the same as EOF, and usually indicates
			// corruption of some sort.
			return chain, disko.NewDriverErrorWithMessage(
				syscall.EINVAL,
				fmt.Sprintf(
					"cluster %d followed by invalid cluster 0x%x at index %d in chain from %d",
					currentCluster,
					nextCluster,
					i,
					chainStart))
		}

		currentCluster = nextCluster
		i++
	}

	return chain, nil

}

// getClusterInChain returns the ID of the `index`th cluster in the chain starting at
// `firstCluster`. Indexing begins at 0. A cluster ID of 0 indicates an error occurred,
// and the Error object in the second return value will indicate what went wrong.
func (drv *FATDriver) getClusterInChain(firstCluster ClusterID, index uint) (ClusterID, error) {
	currentCluster := firstCluster

	for i := uint(0); i < index; i++ {
		nextCluster, err := drv.fs.GetClusterAtIndex(uint(currentCluster))
		if err != nil {
			return 0, err
		}

		if drv.fs.IsEndOfChain(nextCluster) {
			// Hit EOF
			return 0, disko.NewDriverErrorWithMessage(
				syscall.EINVAL,
				fmt.Sprintf(
					"cluster index %d out of bounds -- chain from 0x%x has %d clusters",
					index,
					firstCluster,
					i+1))
		} else if !drv.fs.IsValidCluster(nextCluster) {
			// Hit an invalid cluster. This is not the same as EOF, and usually indicates
			// corruption of some sort.
			return 0, disko.NewDriverErrorWithMessage(
				syscall.EINVAL,
				fmt.Sprintf(
					"cluster %d followed by invalid cluster 0x%x at index %d in chain from %d",
					currentCluster,
					nextCluster,
					i,
					firstCluster))
		}
		currentCluster = nextCluster
	}

	return currentCluster, nil
}

func (drv *FATDriver) readClusterOfDirent(dirent *Dirent, index uint) ([]byte, error) {
	cluster, err := drv.getClusterInChain(dirent.FirstCluster, index)
	if err != nil {
		return nil, err
	}
	return drv.readCluster(cluster, 1)
}

////////////////////////////////////////////////////////////////////////////////////////
// Parts of the Driver interface that can be implemented with little knowledge of the
// underlying file system.

// ReadDirFromDirent returns a list of the directory entries found in directoryDirent,
// including the `.` and `..` entries.
func (drv *FATDriver) ReadDirFromDirent(directoryDirent *Dirent) ([]Dirent, error) {
	if !directoryDirent.IsDir() {
		return nil, disko.NewDriverError(syscall.ENOTDIR)
	}

	bootSector := drv.fs.GetBootSector()
	allDirents := []Dirent{}

	i := uint(0)
	for true {
		clusterData, err := drv.readClusterOfDirent(directoryDirent, i)
		if err != nil {
			return nil, err
		}

		clusterDirents, err := drv.clusterToDirentSlice(clusterData)
		if err != nil {
			return nil, err
		}

		allDirents = append(allDirents, clusterDirents...)
		if len(clusterDirents) < bootSector.DirentsPerCluster {
			break
		}

		i++
	}

	return allDirents, nil
}

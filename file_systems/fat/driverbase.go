package fat

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
)

// This file defines the driver interface and delegates to the underlying version-specific
// drivers.

type FATDriverCommon interface {
	GetBootSector() *FATBootSector
	GetClusterAtIndex(index uint) (ClusterID, error)
	SetClusterAtIndex(index uint, cluster ClusterID) error
	GetNextClusterInChain(cluster ClusterID) (ClusterID, error)
	IsValidCluster(cluster ClusterID) bool
	IsEndOfChain(cluster ClusterID) bool
	ListRootDirectory() ([]Dirent, error)
	AllocateCluster(count uint) ([]ClusterID, error)
	FreeCluster(cluster ClusterID) error
	UpdateDirent(dirent *Dirent) error
	DeleteDirent(dirent, parent *Dirent) error
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
			"unexpected short read. Wanted %d bytes, got %d", bootSector.BytesPerSector, nRead)
	}

	return buffer, nil
}

// readCluster returns the bytes of the `index`th cluster on the file system.
func (drv *FATDriver) readCluster(cluster ClusterID) ([]byte, error) {
	sectorID, err := drv.getFirstSectorOfCluster(cluster)
	if err != nil {
		return nil, err
	}

	bootSector := drv.fs.GetBootSector()
	return drv.readAbsoluteSectors(sectorID, uint(bootSector.SectorsPerCluster))
}

// listClusters returns a list of every cluster in the chain beginning at chainStart.
//
// The returned list will always have chainStart as its first member, unless chainStart
// is an EOF marker (e.g. 0xFFF on FAT12 systems). In this case, the list is empty.
func (drv *FATDriver) listClusters(chainStart ClusterID) ([]ClusterID, error) {
	if !drv.fs.IsValidCluster(chainStart) {
		return nil, errors.NewWithMessage(
			errors.EINVAL,
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
			return chain, errors.NewWithMessage(
				errors.EUCLEAN,
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
			return 0, errors.NewWithMessage(
				errors.EINVAL,
				fmt.Sprintf(
					"cluster index %d out of bounds -- chain from 0x%x has %d clusters",
					index,
					firstCluster,
					i+1))
		} else if !drv.fs.IsValidCluster(nextCluster) {
			// Hit an invalid cluster. This is not the same as EOF, and usually indicates
			// corruption of some sort.
			return 0, errors.NewWithMessage(
				errors.EINVAL,
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
	return drv.readCluster(cluster)
}

////////////////////////////////////////////////////////////////////////////////////////
// Parts of the Driver interface that can be implemented with little knowledge of the
// underlying file system.

// resolvePathToDirent converts a path to the directory entry corresponding to that
// path. If an error occurs, the second return value is an Error object indicating what
// went wrong.
func (drv *FATDriver) resolvePathToDirent(path string) (Dirent, error) {
	cleanedPath := filepath.Clean(path)
	pathParts := filepath.SplitList(cleanedPath)

	if len(pathParts) == 0 {
		// Caller gave us an empty path after components were resolved.
		return Dirent{}, errors.NewWithMessage(
			errors.EINVAL, fmt.Sprintf("file path \"%s\" resolves to empty path", path))
	}

	// Get a listing for the root directory. We need to call a separate function because
	// unlike FAT 32, FAT 12/16 have fixed-size root directories that aren't actually files.
	// Drivers must give us a list of directory entries in the root directory.
	currentDirContents, err := drv.fs.ListRootDirectory()
	if err != nil {
		return Dirent{}, nil
	}

	var currentDirent Dirent

	for _, component := range pathParts {
		for _, entry := range currentDirContents {
			if entry.name == component {
				currentDirent = entry
				currentDirContents, err = drv.readDirFromDirent(&entry)
				if err != nil {
					return Dirent{}, err
				}
			}
		}
	}

	return currentDirent, nil
}

// readDirFromDirent returns a list of the directory entries found in directoryDirent,
// including the `.` and `..` entries.
func (drv *FATDriver) readDirFromDirent(directoryDirent *Dirent) ([]Dirent, error) {
	if !directoryDirent.IsDir() {
		return nil, errors.New(errors.ENOTDIR)
	}

	bootSector := drv.fs.GetBootSector()
	allDirents := []Dirent{}

	i := uint(0)
	for {
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

//////////////////////////////////////////////////////////////////////////////////////////
// IMPLEMENTATION OF PUBLIC DRIVER INTERFACE
//
// Please keep functions in the order they're declared in in disko.ReadingDriver and
// disko.Writing driver.
//////////////////////////////////////////////////////////////////////////////////////////

// Readlink is unsupported on FAT file systems, so calling this function will return an
// error.
func (drv *FATDriver) Readlink(path string) (string, error) {
	return "", errors.New(errors.ENOTSUP)
}

// SameFile determines if two FileInfos reference the same file.
func (drv *FATDriver) SameFile(fi1, fi2 os.FileInfo) bool {
	dirLeft := fi1.Sys().(Dirent)
	dirRight := fi2.Sys().(Dirent)

	return dirLeft.FirstCluster == dirRight.FirstCluster
}

// TODO: Open

// Readdir returns information about all files in the directory pointed to by `path`.
func (drv *FATDriver) Readdir(path string) ([]os.FileInfo, error) {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return nil, err
	}

	if !dirent.IsDir() {
		// TODO: Provide the path in the error message
		return nil, errors.New(errors.ENOTDIR)
	}

	dirContents, err := drv.readDirFromDirent(&dirent)
	if err != nil {
		return nil, err
	}

	fileInfos := make([]os.FileInfo, len(dirContents))
	for i, dir := range dirContents {
		fileInfos[i] = dir
	}

	return fileInfos, nil
}

// ReadFile returns the entire contents of the file at the given path.
func (drv *FATDriver) ReadFile(path string) ([]byte, error) {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return nil, err
	}

	if dirent.IsDir() {
		return nil, errors.New(errors.EISDIR)
	}

	allClusters, err := drv.listClusters(dirent.FirstCluster)
	if err != nil {
		return nil, err
	}

	bs := drv.fs.GetBootSector()
	bytesRemaining := dirent.size

	// Now that we have a list of all the clusters in this file, we can stitch the file
	// together by concatenating their contents. The last cluster may not be completely
	// used.
	buffer := new(bytes.Buffer)
	for _, clusterID := range allClusters {
		clusterContents, err := drv.readCluster(clusterID)
		if err != nil {
			return nil, err
		}

		if bytesRemaining < int64(bs.BytesPerCluster) {
			// This is the last cluster; only write bytesRemaining bytes, not the full
			// cluster.
			_, err = buffer.Write(clusterContents[:bytesRemaining])
		} else {
			// Write the entirety of the cluster to the output buffer. (Either this
			// isn't the last cluster or the dirent is an exact multiple of the cluster
			// size.
			_, err = buffer.Write(clusterContents)
		}

		if err != nil {
			return nil, err
		}

		bytesRemaining -= int64(bs.BytesPerCluster)
	}

	return buffer.Bytes(), nil
}

// Stat returns information about the file or directory at the given path.
func (drv *FATDriver) Stat(path string) (disko.FileStat, error) {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return disko.FileStat{}, err
	}

	return dirent.stat, nil
}

// Lstat returns information about the file or directory at the given path.
//
// Since FAT file systems have no concept of links, this behaves exactly the same as
// Stat.
func (drv *FATDriver) Lstat(path string) (disko.FileStat, error) {
	return drv.Stat(path)
}

// Chmod changes the file mode information for the file at the given path.
//
// The FAT file system has a very limited concept of file modes, so this has little effect.
// FAT only recognizes read-only attributes, so if you want to make a file read-only you
// need to clear the read bit from **all** modes.
//
// This function cannot be used to set any mode flags aside from read-only.
func (drv *FATDriver) Chmod(path string, mode os.FileMode) error {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return err
	}

	// Most file mode flags can be ignored. We have to take the most permissive of mode
	// flags since FAT file systems only have a single read-only flag and don't recognize
	// anything beyond that.
	if (mode & 0b010010010) == 0 {
		// No one has write access
		dirent.stat.ModeFlags &= ^os.FileMode(0b010010010)
	} else {
		dirent.stat.ModeFlags |= 0b010010010
	}

	return drv.fs.UpdateDirent(&dirent)
}

// Chown is unsupported on FAT file systems since they have no concept of ownership.
// This function does nothing, only returns an error.
func (drv *FATDriver) Chown(path string, uid, gid int) error {
	return errors.New(errors.ENOTSUP)
}

// Chtimes changes the last accessed and last modified timestamps of a directory entry.
func (drv *FATDriver) Chtimes(path string, atime, mtime time.Time) error {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return err
	}

	dirent.SetLastAccessedAt(atime)
	dirent.SetLastModifiedAt(mtime)
	return drv.fs.UpdateDirent(&dirent)
}

// Lchown is unsupported on FAT file systems since they have no concept of ownership.
// This function does nothing, only returns an error.
func (drv *FATDriver) Lchown(path string, uid, gid int) error {
	return errors.New(errors.ENOTSUP)
}

// Link does nothing and returns an error since links are unsupported on FAT file systems.
func (drv *FATDriver) Link(oldpath, newpath string) error {
	return errors.New(errors.ENOTSUP)
}

// TODO: Mkdir
// TODO: MkdirAll

// Remove deletes the file at the given path. If you want to delete a directory, use
// RemoveAll.
func (drv *FATDriver) Remove(path string) error {
	dirent, err := drv.resolvePathToDirent(path)
	if err != nil {
		return err
	}

	if dirent.IsDir() {
		return errors.New(errors.EISDIR)
	}

	parentDirent, err := drv.resolvePathToDirent(filepath.Dir(path))
	if err != nil {
		return err
	}

	// Once we get here we can delete the directory entry *first* and then deallocate its
	// clusters. If we were to free the clusters first and then try to delete the dirent
	// but fail, we'd have a dirent pointing at garbage at best, or into a valid cluster
	// chain of another directory entry at worst.
	err = drv.fs.DeleteDirent(&dirent, &parentDirent)
	if err != nil {
		return err
	}

	// Successfully deleted the directory entry. Now we release its clusters.
	fileClusters, err := drv.listClusters(dirent.FirstCluster)
	if err != nil {
		return err
	}

	// Release clusters starting from the end of the chain, not the beginning. This way,
	// if we hit an error then the dirent still points to the beginning of the cluster
	// chain and we don't end up with an orphaned chain that would take up disk space.
	for i := len(fileClusters) - 1; i >= 0; i-- {
		err = drv.fs.FreeCluster(ClusterID(i))
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: RemoveAll
// TODO: Repath

// Symlink does nothing and returns an error since links are unsupported on FAT file
// systems.
func (drv *FATDriver) Symlink(oldpath, newpath string) error {
	return errors.New(errors.ENOTSUP)
}

// TODO: Truncate
// TODO: Create
// TODO: WriteFile

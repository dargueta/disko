package fat8

import (
	"os"
	"strings"

	"github.com/dargueta/disko"
)

////////////////////////////////////////////////////////////////////////////////
// Implementing ReadingDriver interface

// SameFile determines if two files are the same, given basic file information.
func (driver *Driver) SameFile(fi1, fi2 os.FileInfo) bool {
	return strings.EqualFold(fi1.Name(), fi2.Name())
}

// TODO(dargueta): Open(path string) (*os.File, error)
// TODO(dargueta): ReadDir(path string) ([]DirectoryEntry, error)
// TODO(dargueta): ReadFile(path string) ([]byte, error)

func (driver *Driver) Stat(path string) (disko.FileStat, error) {
	normalizedPath := strings.ToUpper(path)

	// We don't really care if there's a leading "/" or not, since there are no
	// directories.
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")

	info, found := driver.dirents[normalizedPath]
	if !found {
		return disko.FileStat{}, disko.ErrNotFound
	}

	// Cluster size is fixed at two clusters per track. Since we know the number
	// of sectors per track, we can determine the number of sectors used by the
	// whole clusters. Keep in mind, though, that the last cluster may only be
	// partially used.
	clusterSectorsUsed := uint(len(info.clusters)) * driver.geometry.SectorsPerCluster
	totalSectors := clusterSectorsUsed - info.UnusedSectorsInLastCluster

	// If the file is binary, then the size is totalSectors because binary files
	// by definition must be a multiple of the sector size.

	var size int64
	if info.IsBinary {
		size = int64(totalSectors) * 128
	} else {
		// TODO(dargueta): Handle text files
		err := disko.ErrNotImplemented.WithMessage("text files not supported yet")
		return disko.FileStat{}, err
	}

	return disko.FileStat{
		DeviceID:    0,
		InodeNumber: uint64(info.index),
		Nlinks:      1,
		ModeFlags:   0o777,
		Size:        size,
		BlockSize:   128,
		NumBlocks:   int64(clusterSectorsUsed - info.UnusedSectorsInLastCluster),
	}, nil
}

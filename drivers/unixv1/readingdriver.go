// This file implements the ReadingDriver interface for the UNIXv1 file system.
package unixv1

import (
	"os"

	"github.com/dargueta/disko"
)

func (driver *UnixV1Driver) SameFile(fi1, fi2 os.FileInfo) bool {
	f1Stat := fi1.Sys().(disko.FileStat)
	f2Stat := fi2.Sys().(disko.FileStat)
	return f1Stat.InodeNumber == f2Stat.InodeNumber
}

// Open(path string) (File, error)
// ReadDir(path string) ([]DirectoryEntry, error)

func (driver *UnixV1Driver) ReadFile(path string) ([]byte, error) {
	inode, err := driver.pathToInode(path)
	if err != nil {
		return nil, err
	}

	handle, err := driver.openFileUsingInode(inode)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, int(inode.Size))
	_, err = handle.Read(buffer)
	return buffer, err
}

func (driver *UnixV1Driver) Stat(path string) (disko.FileStat, error) {
	inumber, err := driver.pathToInumber(path)
	if err != nil {
		return disko.FileStat{}, err
	}

	inode, err := driver.inumberToInode(inumber)
	if err != nil {
		return disko.FileStat{}, err
	}
	return inode.FileStat, err
}

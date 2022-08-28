// This file implements the ReadingDriver interface for the UNIXv1 file system.
package unixv1

import (
	"fmt"
	"os"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
)

func (driver *UnixV1Driver) SameFile(fi1, fi2 os.FileInfo) bool {
	f1Stat := fi1.Sys().(disko.FileStat)
	f2Stat := fi2.Sys().(disko.FileStat)
	return f1Stat.InodeNumber == f2Stat.InodeNumber
}

func (driver *UnixV1Driver) Open(path string) (disko.File, error) {
	return driver.OpenFile(path, disko.O_RDONLY, 0)
}

func (driver *UnixV1Driver) ReadDir(path string) ([]disko.DirectoryEntry, error) {
	inode, err := driver.pathToInode(path)
	if err != nil {
		return nil, err
	}

	if !inode.IsDir() {
		err = errors.NewWithMessage(
			errors.ENOTDIR,
			fmt.Sprintf("`%s` is not a directory", path),
		)
	}

	rawDirBytes, err := driver.getRawContentsUsingInode(inode)
	if err != nil {
		return nil, err
	}

	totalDirents := len(rawDirBytes) / 10
	allDirents := make([]disko.DirectoryEntry, totalDirents)

	for i := 0; i < totalDirents; i++ {
		thisDirent, err := driver.buildDirentFromBytes(rawDirBytes[i*10 : (i+1)*10])
		if err != nil {
			continue
		}
		allDirents[i] = thisDirent.DirectoryEntry
	}

	return allDirents, nil
}

func (driver *UnixV1Driver) ReadFile(path string) ([]byte, error) {
	inode, err := driver.pathToInode(path)
	if err != nil {
		return nil, err
	}

	if !inode.IsFile() {
		err = errors.NewWithMessage(
			errors.EISDIR,
			fmt.Sprintf("`%s` is not a file", path),
		)
	}
	return driver.getRawContentsUsingInode(inode)
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

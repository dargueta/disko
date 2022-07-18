package unixv1

import (
	"encoding/binary"
	"os"
	"time"

	"github.com/dargueta/disko"
)

type RawDirent struct {
	Inumber Inumber
	Name    [8]byte
}

type DirectoryEntry struct {
	disko.DirectoryEntry
	stat Inode
	name string
}

func (dirent *DirectoryEntry) Name() string {
	return dirent.name
}

func (dirent *DirectoryEntry) Size() int64 {
	return dirent.stat.Size
}
func (dirent *DirectoryEntry) Mode() os.FileMode {
	return os.FileMode(dirent.stat.ModeFlags)
}
func (dirent *DirectoryEntry) ModTime() time.Time {
	return dirent.stat.LastModified
}

func (dirent *DirectoryEntry) IsDir() bool {
	return dirent.stat.IsDir()
}

func (dirent *DirectoryEntry) Sys() Inode {
	return dirent.stat
}

func (driver *UnixV1Driver) buildDirentFromBytes(data []byte) (DirectoryEntry, error) {
	inumber := binary.LittleEndian.Uint16(data)
	name := string(data[2:])

	inode, err := driver.inumberToInode(Inumber(inumber))
	if err != nil {
		return DirectoryEntry{}, err
	}

	return DirectoryEntry{
		stat: inode,
		name: name,
	}, nil
}

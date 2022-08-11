package basedriver

import (
	"os"
	"time"

	"github.com/dargueta/disko"
)

type DirectoryEntry struct {
	disko.DirectoryEntry

	// name is the name of the directory entry without its path component.
	name string
	// stat is a copy of the file's status information.
	stat disko.FileStat
}

func NewDirectoryEntryFromHandle(object ObjectHandle) DirectoryEntry {
	return DirectoryEntry{
		name: object.Name(),
		stat: object.Stat(),
	}
}

func (dirent DirectoryEntry) Name() string {
	return dirent.name
}

func (dirent DirectoryEntry) IsDir() bool {
	return dirent.stat.ModeFlags.IsDir()
}

func (dirent DirectoryEntry) Type() os.FileMode {
	return dirent.stat.ModeFlags
}

func (dirent DirectoryEntry) Info() (os.FileInfo, error) {
	return dirent, nil
}

func (dirent DirectoryEntry) Size() int64 {
	return dirent.stat.Size
}

func (dirent DirectoryEntry) Mode() os.FileMode {
	return dirent.stat.ModeFlags
}

func (dirent DirectoryEntry) ModTime() time.Time {
	return dirent.stat.LastModified
}

func (dirent DirectoryEntry) Sys() any {
	return dirent.stat
}

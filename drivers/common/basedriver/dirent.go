package basedriver

import (
	"os"
	"time"

	"github.com/dargueta/disko"
)

type DirectoryEntry struct {
	disko.DirectoryEntry
	name string
	stat disko.FileStat
}

func NewDirectoryEntryFromDescriptor(object ObjectDescriptor) DirectoryEntry {
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

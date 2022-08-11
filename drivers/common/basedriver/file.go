package basedriver

import (
	"io/fs"
	"os"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/drivers/common"
	"github.com/dargueta/disko/drivers/common/basicstream"
	"github.com/dargueta/disko/drivers/common/blockcache"
)

type FileInfo struct {
	os.FileInfo
	disko.DirectoryEntry

	disko.FileStat
	name string
}

func (info *FileInfo) Name() string {
	return info.name
}

func (info *FileInfo) Size() int64 {
	return info.FileStat.Size
}

func (info FileInfo) Mode() os.FileMode {
	return os.FileMode(info.FileStat.ModeFlags)
}

func (info *FileInfo) Type() fs.FileMode {
	return info.FileStat.ModeFlags
}

func (info FileInfo) ModTime() time.Time {
	return info.FileStat.LastModified
}

func (info FileInfo) IsDir() bool {
	return info.FileStat.ModeFlags&os.ModeDir != 0
}

func (info *FileInfo) Info() (os.FileInfo, error) {
	return info, nil
}

func (info *FileInfo) Stat() disko.FileStat {
	return info.FileStat
}

func (info *FileInfo) Sys() any {
	return info.FileStat
}

////////////////////////////////////////////////////////////////////////////////

type File struct {
	// Embed
	*basicstream.BasicStream

	// Fields
	owningDriver *CommonDriver
	objectHandle ObjectHandle
	fileInfo     FileInfo
	ioFlags      disko.IOFlags
}

// NewFileFromObjectHandle creates a Disko file object that is (more or less) a
// drop-in replacement for os.File.
func NewFileFromObjectHandle(
	driver *CommonDriver,
	object ObjectHandle,
	ioFlags disko.IOFlags,
) (File, error) {
	fetchCb := func(index common.LogicalBlock, buffer []byte) error {
		return object.ReadBlocks(index, buffer)
	}
	flushCb := func(index common.LogicalBlock, buffer []byte) error {
		return object.WriteBlocks(index, buffer)
	}
	resizeCb := func(newSize common.LogicalBlock) error {
		return object.Resize(uint64(newSize))
	}

	stat := object.Stat()
	blockCache := blockcache.New(
		uint(stat.BlockSize),
		uint(stat.NumBlocks),
		fetchCb,
		flushCb,
		resizeCb,
	)

	stream, err := basicstream.New(stat.Size, blockCache)
	if err != nil {
		return File{}, err
	}

	return File{
		owningDriver: driver,
		objectHandle: object,
		ioFlags:      ioFlags,
		BasicStream:  stream,
		fileInfo: FileInfo{
			FileStat: stat,
			name:     object.Name(),
		},
	}, nil
}

/*
	Chdir					DONE
	Chmod					DONE
	Chown					DONE
	Close					DONE
	Name					DONE
	Read					DONE
	ReadAt					DONE
	Readdir
	Readdirnames
	ReadFrom
	Seek					DONE
	Stat					DONE
	Sync					DONE
	Truncate				DONE
	Write					DONE
	WriteAt					DONE
	WriteString				DONE
*/

func (file *File) Chdir() error {
	return file.owningDriver.chdirToObject(
		file.objectHandle,
		file.fileInfo.name,
	)
}

func (file *File) Chmod(mode os.FileMode) error {
	return file.objectHandle.Chmod(mode)
}

func (file *File) Chown(uid, gid int) error {
	return file.objectHandle.Chown(uid, gid)
}

func (file *File) Close() error {
	file.BasicStream.Close()
	return file.owningDriver.implementation.MarkFileClosed(file)
}

func (file *File) Name() string {
	return file.fileInfo.name
}

func (file *File) Stat() (os.FileInfo, error) {
	return file.fileInfo.Info()
}

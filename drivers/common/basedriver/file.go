package basedriver

import (
	"os"
	posixpath "path"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/drivers/common"
	"github.com/dargueta/disko/drivers/common/basicstream"
	"github.com/dargueta/disko/drivers/common/blockcache"
)

// FileInfo gives detailed information about a file or directory. It implements
// both the [os.FileInfo] and [os.DirEntry] interfaces, and can be used as a
// [disko.FileStat] object as well.
type FileInfo struct {
	// Interfaces
	os.FileInfo
	disko.DirectoryEntry

	// Embedded structs
	disko.FileStat

	// Fields
	absolutePath string
}

// os.FileInfo implementation --------------------------------------------------

// Mode returns the mode flags for the file or directory. It's functionally
// identical to Type(), but used to implement the [os.FileInfo] interface.
func (info FileInfo) Mode() os.FileMode {
	return info.FileStat.ModeFlags
}

func (info *FileInfo) Size() int64 {
	return info.FileStat.Size
}

func (info FileInfo) ModTime() time.Time {
	return info.FileStat.LastModified
}

func (info *FileInfo) Sys() any {
	return info.FileStat
}

// os.DirEntry implementation --------------------------------------------------

func (info *FileInfo) Name() string {
	return posixpath.Base(info.absolutePath)
}

// Type returns the mode flags for the file or directory. It's functionally
// identical to Mode(), but used to implement the [os.DirEntry] interface.
func (info *FileInfo) Type() os.FileMode {
	return info.FileStat.ModeFlags
}

func (info FileInfo) IsDir() bool {
	return info.FileStat.ModeFlags&os.ModeDir != 0
}

// Info is part of the [os.DirEntry] interface. It returns the `FileInfo` it was
// called on, since that implements both interfaces.
func (info *FileInfo) Info() (os.FileInfo, error) {
	return info, nil
}

// disko.DirectoryEntry methods ------------------------------------------------

func (info *FileInfo) Stat() disko.FileStat {
	return info.FileStat
}

////////////////////////////////////////////////////////////////////////////////

type File struct {
	// Embed
	*basicstream.BasicStream

	// Fields
	owningDriver *CommonDriver
	objectHandle extObjectHandle
	fileInfo     FileInfo
	ioFlags      disko.IOFlags
}

// NewFileFromObjectHandle creates a Disko file object that is (more or less) a
// drop-in replacement for [os.File].
func NewFileFromObjectHandle(
	driver *CommonDriver,
	object extObjectHandle,
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

	stream, err := basicstream.New(stat.Size, blockCache, ioFlags)
	if err != nil {
		return File{}, err
	}

	return File{
		owningDriver: driver,
		objectHandle: object,
		ioFlags:      ioFlags,
		BasicStream:  stream,
		fileInfo: FileInfo{
			FileStat:     stat,
			absolutePath: object.AbsolutePath(),
		},
	}, nil
}

func (file *File) Chdir() error {
	return file.owningDriver.chdirToObject(file.objectHandle)
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

package driver

import (
	"io"
	"os"
	posixpath "path"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
	"github.com/dargueta/disko/file_systems/common"
	"github.com/dargueta/disko/file_systems/common/basicstream"
	"github.com/dargueta/disko/file_systems/common/blockcache"
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

// ModTime returns the timestamp of when the file was last modified. If the file
// system doesn't record this information, implementations MUST return zero time
func (info FileInfo) ModTime() time.Time {
	return info.FileStat.LastModified
}

func (info *FileInfo) Sys() interface{} {
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
	owningDriver *Driver
	objectHandle extObjectHandle
	fileInfo     FileInfo
	ioFlags      disko.IOFlags

	lastReadDirResult    []disko.DirectoryEntry
	readDirResultPointer int
}

// NewFileFromObjectHandle creates a Disko file object that is (more or less) a
// drop-in replacement for [os.File].
func NewFileFromObjectHandle(
	driver *Driver,
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
	return file.BasicStream.Close()
}

func (file *File) Name() string {
	return file.objectHandle.Name()
}

func (file *File) ReadDir(n int) ([]os.DirEntry, error) {
	stat := file.objectHandle.Stat()
	if !stat.IsDir() {
		return nil, errors.New(errors.ENOTDIR)
	}

	if file.lastReadDirResult == nil {
		// The function has never been called or was exhausted on a previous
		// call. Read the contents of the directory and set up the queue.
		entries, err := file.owningDriver.readDir(file.objectHandle)
		if err != nil {
			return nil, err
		}

		file.lastReadDirResult = entries
		file.readDirResultPointer = 0
	}

	entriesRemaining := len(file.lastReadDirResult) - file.readDirResultPointer
	var numToCopy int
	if n <= 0 || n > entriesRemaining {
		numToCopy = entriesRemaining
	} else {
		numToCopy = n
	}

	result := make([]os.DirEntry, numToCopy)

	// If there are no entries remaining, return an empty slice and io.EOF.
	if entriesRemaining == 0 {
		file.lastReadDirResult = nil
		file.readDirResultPointer = 0
		return result, io.EOF
	}

	// TODO (dargueta): Is there a way to use copy() for a slice of a superset interface?
	// It shouldn't be a performance problem but this feels clunky.
	for i := 0; i < numToCopy; i++ {
		result[i] = file.lastReadDirResult[file.readDirResultPointer]
		file.readDirResultPointer++
	}
	return result, nil
}

func (file *File) Readdir(n int) ([]os.FileInfo, error) {
	dirents, err := file.ReadDir(n)
	if err == io.EOF {
		// If we hit EOF, return an empty slice, not nil.
		return make([]os.FileInfo, 0), err
	} else if err != nil {
		// Unknown error
		return nil, err
	}

	infoList := make([]os.FileInfo, len(dirents))
	for i, dirent := range dirents {
		infoList[i], err = dirent.Info()
		if err != nil {
			// Hit an error, return what we have so far instead of tossing the
			// entire result.
			return infoList[:i], err
		}
	}
	return infoList, nil
}

func (file *File) Readdirnames(n int) ([]string, error) {
	dirents, err := file.ReadDir(n)
	if err == io.EOF {
		// If we hit EOF, return an empty slice not nil.
		return make([]string, 0), err
	} else if err != nil {
		// Unknown error
		return nil, err
	}

	names := make([]string, len(dirents))
	for i, dirent := range dirents {
		names[i] = dirent.Name()
	}
	return names, nil
}

func (file *File) Stat() (os.FileInfo, error) {
	return file.fileInfo.Info()
}

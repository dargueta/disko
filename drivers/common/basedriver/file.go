package basedriver

import (
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/dargueta/disko"
)

type FileInfo struct {
	fs.FileInfo
	disko.FileStat
	name string
}

func (info *FileInfo) Name() string {
	return info.name
}

func (info *FileInfo) Size() int64 {
	return info.FileStat.Size
}

func (info *FileInfo) Mode() fs.FileMode {
	return fs.FileMode(info.FileStat.ModeFlags)
}

func (info *FileInfo) ModTime() time.Time {
	return info.FileStat.LastModified
}

func (info *FileInfo) IsDir() bool {
	return info.FileStat.ModeFlags&disko.S_IFDIR != 0
}

func (info *FileInfo) Sys() disko.FileStat {
	return info.FileStat
}

////////////////////////////////////////////////////////////////////////////////

type File struct {
	io.ReadWriteCloser
	io.Seeker
	io.ReaderAt
	io.ReaderFrom
	io.WriterAt
	io.StringWriter
	disko.Truncator

	disko.File
	owningDriver *CommonDriver
	fileInfo     FileInfo
	ioFlags      disko.IOFlags
}

/*
	io.ReadWriteCloser
	io.Seeker
	io.ReaderAt
	io.ReaderFrom
	io.WriterAt
	io.StringWriter
	Truncator

	Chdir() error
	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
	Fd() uintptr 							// DONE
	Name() string							// DONE
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	SetDeadline(t time.Time) error 			// DONE
	SetReadDeadline(t time.Time) error 		// DONE
	SetWriteDeadline(t time.Time) error 	// DONE
	SyscallConn() (syscall.RawConn, error) 	// DONE
	Stat() (os.FileInfo, error) 			// DONE
	Sync()
*/

func (file *File) Close() error {
	return file.owningDriver.implementation.MarkFileClosed(file)
}

func (file *File) Fd() uintptr {
	return 0
}

func (file *File) SetDeadline(t time.Time) error {
	return nil
}

func (file *File) SetReadDeadline(t time.Time) error {
	return nil
}

func (file *File) SetWriteDeadline(t time.Time) error {
	return nil
}

func (file *File) SyscallConn() (syscall.RawConn, error) {
	return nil, disko.NewDriverError(disko.ENOSYS)
}

func (file *File) Stat() (os.FileInfo, error) {
	return file.fileInfo.FileInfo, nil
}

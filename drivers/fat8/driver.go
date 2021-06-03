package fat8

import (
	"bytes"
	"fmt"
	"os"

	"github.com/dargueta/disko"
)

type FAT8Driver struct {
	disko.ReadingDriver
	disko.WritingDriver
	disko.FormattingDriver
	sectorsPerTrack int
	totalTracks     int
	infoSectorIndex int
	image           *os.File
	stat            disko.FSStat
}

func (driver *FAT8Driver) Format(information disko.FSStat) error {
	if information.TotalBlocks == 2002 {
		driver.sectorsPerTrack = 26
		driver.totalTracks = 77
		driver.infoSectorIndex = 19
	} else if information.TotalBlocks == 720 {
		driver.sectorsPerTrack = 18
		driver.totalTracks = 40
		driver.infoSectorIndex = 12
	} else {
		return fmt.Errorf(
			"invalid format configuration: TotalBlocks must be 2002 or 720, got %d",
			information.TotalBlocks)
	}

	// Create a blank image filled with null bytes
	driver.image.Seek(0, 0)
	driver.image.Truncate(0)
	driver.image.Write(bytes.Repeat([]byte{0}, int(128*driver.sectorsPerTrack*driver.totalTracks)))
	trackSize := 128 * driver.sectorsPerTrack

	// The directory track is in the middle of the disk.
	directoryTrackNumber := driver.totalTracks / 2
	directoryTrackStart := trackSize * directoryTrackNumber

	// There are two clusters per track, so the size of the FAT is one byte per
	// cluster plus some padding bytes to get to a multiple of the sector size.
	fatSize := int(driver.totalTracks * 2)
	if fatSize%128 != 0 {
		fatSize += 128 - (fatSize % 128)
	}

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	fat := bytes.Repeat([]byte{0xff}, fatSize)
	fat[directoryTrackNumber*2] = 0xfe
	fat[directoryTrackNumber*2+1] = 0xfe

	fatStart := directoryTrackStart + trackSize - (3 * fatSize)
	driver.image.Seek(int64(fatStart), 0)

	_, err := driver.image.Write(bytes.Repeat(fat, 3))
	return err
}

func FormatMinifloppyImage(image *os.File) error {
	image.Seek(0, 0)
	image.Truncate(0)
	image.Write(bytes.Repeat([]byte{0}, 92160))

	return nil
}

func FormatFloppyImage(image *os.File) error {
	image.Seek(0, 0)
	image.Truncate(0)
	image.Write(bytes.Repeat([]byte{0}, 256256))

	return nil
}

/*
// OpeningDriver is the interface for drivers implementing the POSIX open(3) function.
//
// Drivers need not implement all functionality for valid flags. For example, read-only
// drivers must return an error if called with the os.O_CREATE flag.
type OpeningDriver interface {
	// OpenFile is equivalent to os.OpenFile.
	OpenFile(path string, flag int, perm os.FileMode) (*os.File, error)
}

// ReadingDriver is the interface for drivers supporting read operations.
type ReadingDriver interface {
	Readlink(path string) (string, error)
	SameFile(fi1, fi2 os.FileInfo) bool
	Open(path string) (*os.File, error)
	ReadDir(path string) ([]os.FileInfo, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use a
	// reasonable default value. For most of these 0 is fine, but for compatibility
	// drivers should use 1 for `Nlink` and 0o777 for `Mode`.
	Stat(path string) (disko.FileStat, error)
	// Lstat returns the same information as Stat but follows symbolic links. On file
	// systems that don't support symbolic links, the behavior is exactly the same as
	// Stat.
	Lstat(path string) (disko.FileStat, error)
}

// WritingDriver is the interface for drivers supporting write operations.
type WritingDriver interface {
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Repath(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (*os.File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
}
*/

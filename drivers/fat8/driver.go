package fat8

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/dargueta/disko"
)

type FAT8FileInfo struct {
	disko.DirectoryEntry
	name                       string
	clusters                   []uint8
	index                      uint
	IsBinary                   bool
	IsEBCDIC                   bool
	IsWriteProtected           bool
	ReadAfterWriteEnabled      bool
	UnusedSectorsInLastCluster uint
}

type FAT8Driver struct {
	disko.ReadingDriver
	disko.WritingDriver
	disko.FormattingDriver
	// image is a file object for the file the disk image is for.
	image *os.File
	// sectorsPerTrack defines the number of sectors in a single track for the
	// current disk geometry.
	sectorsPerTrack uint
	// totalTracks gives the number of tracks for the current disk geometry.
	totalTracks uint
	// bytesPerCluster gives the number of bytes in a single cluster.
	bytesPerCluster uint
	// infoSectorIndex is the zero-based index of the "information sector" of a
	// FAT8 image.
	//
	// To my knowledge the FAT8 standard only defines the first byte of this
	// sector, which is the default attribute byte to use for new files.
	infoSectorIndex      uint
	defaultFileAttrFlags uint8
	stat                 disko.FSStat
	// freeClusters is an array of the indexes of all unallocated clusters. This
	// will never be more than 189 entries long.
	freeClusters []uint8
	// fat is the FAT as represented on the disk. It's always a multiple of 128
	// in length, but only the first totalTracks*2 entries are valid. Anything
	// beyond that must not be modified.
	fat []uint8
	// isMounted indicates if the drive is currently mounted.
	isMounted bool
	dirents   map[string]FAT8FileInfo
}

////////////////////////////////////////////////////////////////////////////////
// General utility functions

func (driver *FAT8Driver) trackAndSectorToFileOffset(track, sector uint) (int64, error) {
	if track >= driver.totalTracks {
		return -1,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid track number: %d not in [0, %d)",
					track,
					driver.totalTracks,
				),
			)
	}
	if sector >= driver.sectorsPerTrack {
		return -1,
			disko.NewDriverErrorWithMessage(
				disko.EINVAL,
				fmt.Sprintf(
					"invalid sector number: %d not in [0, %d)",
					sector,
					driver.sectorsPerTrack,
				),
			)
	}

	absoluteSector := (track * driver.sectorsPerTrack) + sector
	return int64(absoluteSector * 128), nil
}

func (driver *FAT8Driver) readSectors(track, sector, count uint) ([]byte, error) {
	offset, err := driver.trackAndSectorToFileOffset(track, sector)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 128*count)

	driver.image.ReadAt(buffer, offset)
	_, err = driver.image.Read(buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func (driver *FAT8Driver) ReadCluster(clusterID uint) ([]byte, error) {
	if (clusterID < 1) || (clusterID >= 0xc0) {
		return nil,
			fmt.Errorf("bad cluster number: %#x not in [1, 0xc0)", clusterID)
	}
	track := (clusterID - 1) / 2
	trackCluster := (clusterID - 1) % 2
	startingSector := trackCluster * driver.sectorsPerTrack / 2
	return driver.readSectors(track, startingSector, driver.sectorsPerTrack/2)
}

func (driver *FAT8Driver) writeSectors(track, sector uint, data []byte) error {
	offset, err := driver.trackAndSectorToFileOffset(track, sector)
	if err != nil {
		return err
	}

	imageSize := driver.stat.TotalBlocks * 128
	if uint64(offset)+uint64(len(data)) >= imageSize {
		return disko.NewDriverErrorWithMessage(
			disko.EINVAL,
			fmt.Sprintf(
				"can't write %d bytes at track %d, sector %d: extends past end of image",
				len(data),
				track,
				sector,
			),
		)
	}

	_, err = driver.image.WriteAt(data, offset)
	return err
}

// WriteCluster writes bytes to the given cluster. `data` must be exactly the
// size of a cluster.
func (driver *FAT8Driver) WriteCluster(clusterID uint, data []byte) error {
	if (clusterID < 1) || (clusterID >= 0xc0) {
		return fmt.Errorf("bad cluster number: %#x not in [1, 0xc0)", clusterID)
	}
	if len(data) != int(driver.bytesPerCluster) {
		return fmt.Errorf(
			"data to write is the wrong size: expected %d, got %d",
			driver.bytesPerCluster,
			len(data))
	}

	track := (clusterID - 1) / 2
	trackCluster := (clusterID - 1) % 2
	startingSector := trackCluster * driver.sectorsPerTrack / 2
	return driver.writeSectors(track, startingSector, data)
}

func (driver *FAT8Driver) readFATs() ([]byte, error) {
	// The directory track is always the middle one in the disk.
	directoryTrack := driver.totalTracks / 2

	// Each track is two clusters, so the number of entries in the FAT is two
	// times the number of tracks. Each entry is one byte, so if we do a ceiling
	// division of the number of bytes in a FAT by 128, we'll get the FAT size
	// in sectors.
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := uint((totalClusters + (-totalClusters % 128)) / 128)

	// There are three copies of the FAT at the end of the directory track. Read
	// each one and ensure they're all identical. If they're not, that's likely
	// an indicator of disk corruption.
	firstFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*3),
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	secondFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-(fatSizeInSectors*2),
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	thirdFAT, err := driver.readSectors(
		directoryTrack,
		driver.sectorsPerTrack-fatSizeInSectors,
		fatSizeInSectors)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(firstFAT, secondFAT) {
		return nil, disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 2")
	} else if !bytes.Equal(firstFAT, thirdFAT) {
		return nil, disko.NewDriverErrorWithMessage(
			disko.EUCLEAN,
			"disk corruption detected: FAT copy 1 differs from FAT copy 3")
	}

	return firstFAT, nil
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *FAT8Driver) Mount(flags disko.MountFlags) error {
	// Ignore attempts to mount the drive multiple times.
	if driver.isMounted {
		return disko.NewDriverError(disko.EALREADY)
	}

	offset, err := driver.image.Seek(0, 2)
	if err != nil {
		return err
	}

	if offset == 256256 {
		driver.sectorsPerTrack = 26
		driver.totalTracks = 77
		driver.infoSectorIndex = 19
	} else if offset == 92160 {
		driver.sectorsPerTrack = 18
		driver.totalTracks = 40
		driver.infoSectorIndex = 12
	} else {
		message := fmt.Sprintf(
			"invalid disk image size; expected 256256 or 92160, got %d",
			offset)
		return disko.NewDriverErrorWithMessage(disko.EMEDIUMTYPE, message)
	}

	driver.bytesPerCluster = driver.sectorsPerTrack * 64

	// All FATs are identical, so we only need to store the first one.
	fat, err := driver.readFATs()
	if err != nil {
		return err
	}
	driver.fat = fat

	// Build a list of all currently free clusters.
	for i, clusterNumber := range fat {
		if clusterNumber == 0xff {
			driver.freeClusters = append(driver.freeClusters, uint8(i+1))
		}
	}

	// The directory track is always the middle one in the disk.
	directoryTrack := driver.totalTracks / 2

	// Get the info sector, which is always at a fixed location in the directory
	// track. The first byte of the info sector tells us what the default
	// attributes should be for new files; the rest is not defined by the standard.
	infoSector, err := driver.readSectors(directoryTrack, driver.infoSectorIndex, 1)
	if err != nil {
		return err
	}
	driver.defaultFileAttrFlags = infoSector[0]

	return nil
}

// TODO (dargueta): Unmount()

func (driver *FAT8Driver) GetFSInfo() disko.FSStat {
	return driver.stat
}

////////////////////////////////////////////////////////////////////////////////
// Implementing FormattingDriver interface

// Format creates a new empty disk image using the given disk information.
//
// This driver only requires the TotalBlocks field to be set in `information`.
// It must either be 2002 for a floppy image, or 720 for a minifloppy image.
func (driver *FAT8Driver) Format(information disko.FSStat) error {
	if driver.isMounted {
		return disko.NewDriverErrorWithMessage(
			disko.EBUSY,
			"image must be unmounted before it can be formatted")
	}

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
	fileSize := 128 * information.TotalBlocks
	driver.image.Seek(0, 0)
	driver.image.Truncate(int64(fileSize))
	driver.image.Write(bytes.Repeat([]byte{0}, int(fileSize)))

	// There are two clusters per track, so the size of the FAT is one byte per
	// cluster plus some padding bytes to get to a multiple of the sector size.
	totalClusters := int(driver.totalTracks * 2)
	fatSizeInSectors := (totalClusters + (-totalClusters % -128)) / 128

	// The directory track is in the middle of the disk.
	directoryTrackNumber := driver.totalTracks / 2

	// Construct a single copy of the FAT, and mark the directory track as
	// reserved by putting 0xFE in the cluster entry. (It's always the middle
	// track.)
	fat := bytes.Repeat([]byte{0xff}, fatSizeInSectors*128)
	fat[directoryTrackNumber*2] = 0xfe
	fat[directoryTrackNumber*2+1] = 0xfe

	allFATs := bytes.Repeat(fat, 3)

	// Write the FATs
	fatStart := driver.sectorsPerTrack - uint(fatSizeInSectors*3)
	err := driver.writeSectors(directoryTrackNumber, fatStart, allFATs)

	if err != nil {
		return err
	}

	// We reserve one track for the directory, so the total number of available
	// blocks is one track's worth of blocks fewer.
	availableBlocks := information.TotalBlocks - uint64(driver.sectorsPerTrack)

	// The maximum number of files is:
	// (SectorsPerTrack - 1 - (FatSizeInSectors * 3)) * DirentsPerSector
	//   * We subtract one for the information sector.
	//   * A directory entry is 16 bytes, so there are 8 dirents per sector.
	direntSectors := driver.sectorsPerTrack - 1 - uint(fatSizeInSectors*3)
	totalDirents := direntSectors * 8

	driver.stat = disko.FSStat{
		BlockSize:       128,
		TotalBlocks:     information.TotalBlocks,
		BlocksFree:      availableBlocks,
		BlocksAvailable: availableBlocks,
		Files:           0,
		FilesFree:       uint64(totalDirents),
		// This isn't completely accurate; names are 6.3 format so the longest
		// bare name is six characters, plus an extra three for the extension,
		// plus one more for the ".". Problem is, "ABCDEFGHI" is interpreted as
		// "ABCDEF.GHI" because of the implicit period.
		MaxNameLength: 10,
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Implementing FormattingDriver interface

// SameFile determines if two files are the same, given basic file information.
func (driver *FAT8Driver) SameFile(fi1, fi2 os.FileInfo) bool {
	return strings.EqualFold(fi1.Name(), fi2.Name())
}

// TODO(dargueta): Open(path string) (*os.File, error)
// TODO(dargueta): ReadDir(path string) ([]DirectoryEntry, error)
// TODO(dargueta): ReadFile(path string) ([]byte, error)

func (driver *FAT8Driver) Stat(path string) (disko.FileStat, error) {
	normalizedPath := strings.ToUpper(path)

	// We don't really care if there's a leading "/" or not, since there are no
	// directories.
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")

	info, found := driver.dirents[normalizedPath]
	if !found {
		return disko.FileStat{}, disko.NewDriverError(disko.ENOENT)
	}

	// Cluster size is fixed at two clusters per track. Since we know the number
	// of sectors per track, we can determine the number of sectors used by the
	// whole clusters. Keep in mind, though, that the last cluster may only be
	// partially used.
	clusterSectorsUsed := uint(len(info.clusters)) * driver.sectorsPerTrack / 2
	totalSectors := clusterSectorsUsed - info.UnusedSectorsInLastCluster

	// If the file is binary, then the size is totalSectors because binary files
	// by definition must be a multiple of the sector size.

	var size int64
	if info.IsBinary {
		size = int64(totalSectors) * 128
	} else {
		// TODO(dargueta): Handle text files
		err := disko.NewDriverErrorWithMessage(disko.ENOSYS, "text files not supported yet")
		return disko.FileStat{}, err
	}

	return disko.FileStat{
		DeviceID:    0,
		InodeNumber: uint64(info.index),
		Nlinks:      1,
		ModeFlags:   0o777,
		Size:        size,
		BlockSize:   128,
		Blocks:      int64(clusterSectorsUsed - info.UnusedSectorsInLastCluster),
	}, nil
}

/*

// FormattingDriver is the interface for drivers capable of creating new disk
// images.

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
	Open(path string) (*os.File, error)
	ReadDir(path string) ([]DirectoryEntry, error)
	// ReadFile return the contents of the file at the given path.
	ReadFile(path string) ([]byte, error)
	// Stat returns information about the directory entry at the given path.
	//
	// If a file system doesn't support a particular feature, drivers should use a
	// reasonable default value. For most of these 0 is fine, but for compatibility
	// drivers should use 1 for `Nlinks` and 0o777 for `ModeFlags`.
	Stat(path string) (FileStat, error)
}

// ReadingLinkingDriver provides a read-only interface for linking features on
// file systems that support links.
type ReadingLinkingDriver interface {
	Readlink(path string) (string, error)

	// Lstat returns the same information as Stat but follows symbolic links.
	Lstat(path string) (FileStat, error)
}

// WritingDriver is the interface for drivers supporting write operations.
type WritingDriver interface {
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Repath(oldpath, newpath string) error
	Truncate(path string, size int64) error
	Create(path string) (*os.File, error)
	WriteFile(filepath string, data []byte, perm os.FileMode) error
	// Flush writes all changes to the disk image.
	Flush() error
}

// WritingLinkingDriver provides a writing interface to linking features on file
// systems that support links.
type WritingLinkingDriver interface {
	Lchown(path string, uid, gid int) error
	Link(oldpath, newpath string) error
	Symlink(oldpath, newpath string) error
}
*/

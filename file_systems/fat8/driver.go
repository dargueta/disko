package fat8

import (
	"io"
	"os"

	"github.com/dargueta/disko"
)

type LogicalBlock uint
type PhysicalBlock uint
type LogicalCluster uint
type PhysicalCluster uint

type DirectoryEntry struct {
	disko.DirectoryEntry
	name                       string
	clusters                   []PhysicalCluster
	index                      uint
	IsBinary                   bool
	IsEBCDIC                   bool
	IsWriteProtected           bool
	ReadAfterWriteEnabled      bool
	UnusedSectorsInLastCluster uint
}

type Driver struct {
	disko.ReadingDriver
	disko.WritingDriver
	disko.FormattingDriver
	// image is a file object for the file the disk image is for.
	image                *os.File
	geometry             Geometry
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
	dirents   map[string]DirectoryEntry
}

func NewDriverFromFile(stream *os.File) Driver {
	return Driver{image: stream}
}

////////////////////////////////////////////////////////////////////////////////
// Implementing Driver interface

func (driver *Driver) Mount(flags disko.MountFlags) error {
	// Ignore attempts to mount the drive multiple times.
	if driver.isMounted {
		return disko.NewDriverError(disko.EALREADY)
	}

	offset, err := driver.image.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	geo, err := GetGeometry(uint(offset) / 128)
	if err != nil {
		return disko.NewDriverErrorWithMessage(disko.EUCLEAN, err.Error())
	}
	driver.geometry = geo

	// All FATs are identical, so we only need to store the first one.
	fat, err := driver.GetFAT()
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

	// Get the info sector, which always immediately precedes the FATs. The first
	// byte of the info sector tells us what the default attributes should be for
	// new files; the rest is not defined by the standard.
	infoSector, err := driver.ReadDiskBlocks(driver.geometry.InfoSectorStart, 1)
	if err != nil {
		return err
	}
	driver.defaultFileAttrFlags = infoSector[0]
	driver.isMounted = true
	return nil
}

func (driver *Driver) Unmount() error {
	err := driver.writeFAT()
	if err != nil {
		return err
	}
	driver.isMounted = false
	return nil
}

func (driver *Driver) GetFSInfo() disko.FSStat {
	return driver.stat
}

package unixv1

import (
	"fmt"

	"github.com/dargueta/disko"
)

// ConvertFSFlagsToStandard takes inode flags found in the on-disk representation
// of an inode and converts them to their standardized Unix equivalents. For
// example, `FlagNonOwnerRead` is converted to `S_IRGRP | S_IROTH`. Unrecognized
// flags are ignored.
func ConvertFSFlagsToStandard(rawFlags uint16) uint32 {
	stdFlags := uint32(0)

	if rawFlags&FlagIsDirectory != 0 {
		// N.B. directories must be marked executable on modern *NIX systems.
		stdFlags |= disko.S_IFDIR | disko.S_IXUSR | disko.S_IXGRP | disko.S_IXOTH
	}
	if rawFlags&FlagSetUIDOnExecution != 0 {
		stdFlags |= disko.S_ISUID
	}
	if rawFlags&FlagIsExecutable != 0 {
		stdFlags |= disko.S_IXUSR | disko.S_IXGRP | disko.S_IXOTH
	}
	if rawFlags&FlagOwnerRead != 0 {
		stdFlags |= disko.S_IRUSR
	}
	if rawFlags&FlagOwnerWrite != 0 {
		stdFlags |= disko.S_IWUSR
	}
	if rawFlags&FlagNonOwnerRead != 0 {
		stdFlags |= disko.S_IRGRP | disko.S_IROTH
	}
	if rawFlags&FlagNonOwnerWrite != 0 {
		stdFlags |= disko.S_IWGRP | disko.S_IWOTH
	}

	return stdFlags
}

// ConvertStandardFlagsToFS is the inverse of ConvertFSFlagsToStandard; it takes
// Unix mode flags and converts them to their on-disk representation.
func ConvertStandardFlagsToFS(flags uint32) uint16 {
	rawFlags := uint16(0)

	if flags&disko.S_IFDIR != 0 {
		rawFlags |= FlagIsDirectory
	}
	if flags&disko.S_ISUID != 0 {
		rawFlags |= FlagSetUIDOnExecution
	}
	if flags&disko.S_IRUSR != 0 {
		rawFlags |= FlagOwnerRead
	}
	if flags&disko.S_IWUSR != 0 {
		rawFlags |= FlagOwnerWrite
	}
	if flags&(disko.S_IRGRP|disko.S_IROTH) != 0 {
		rawFlags |= FlagNonOwnerRead
	}
	if flags&(disko.S_IWGRP|disko.S_IWOTH) != 0 {
		rawFlags |= FlagNonOwnerWrite
	}

	// Only mark a dirent as executable if it's got execution permissions AND
	// isn't a directory.
	if flags&(disko.S_IXUSR|disko.S_IFDIR) == disko.S_IXUSR {
		rawFlags |= FlagIsExecutable
	}
	return rawFlags
}

func (driver *UnixV1Driver) pathToInumber(path string) (Inumber, error) {
	return Inumber(0), fmt.Errorf("pathToInumber() not implemented")
}

func (driver *UnixV1Driver) inumberToInode(inumber Inumber) (Inode, error) {
	return Inode{}, fmt.Errorf("inumberToInode() not implemented")
}

func (driver *UnixV1Driver) pathToInode(path string) (Inode, error) {
	inumber, err := driver.pathToInumber(path)
	if err != nil {
		return Inode{}, err
	}
	return driver.inumberToInode(inumber)
}

func (driver *UnixV1Driver) openFileUsingInode(inode Inode) (disko.File, error) {
	return nil, fmt.Errorf("openFileWithInode() not implemented")
}

func (driver *UnixV1Driver) getRawContentsUsingInode(inode Inode) ([]byte, error) {
	handle, err := driver.openFileUsingInode(inode)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, int(inode.Size))
	_, readErr := handle.Read(buffer)
	closeErr := handle.Close()

	if readErr != nil {
		return nil, readErr
	} else if closeErr != nil {
		return nil, closeErr
	}
	return buffer, nil
}

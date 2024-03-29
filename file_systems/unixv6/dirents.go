package unixv6

import (
	"fmt"
	"os"
	"time"

	"github.com/dargueta/disko"
)

type Inumber uint16
type BlockNum uint16

type RawSuperblock struct {
	NumInodeBlocks       uint16        // isize
	TotalBlocks          uint16        // fsize
	NumFreeListEntries   uint16        // nfree
	FreeList             [100]BlockNum // free
	NumIlistEntries      uint16        // ninode
	InumberList          [100]Inumber  // inode
	FLock                uint8         // flock
	ILock                uint8         // ilock
	FModFlags            uint8         // fmod
	SuperblockModifiedAt uint32        // time
}

type RawInode struct {
	Flags        uint16
	NLink        uint8
	UID          uint8
	GID          uint8
	Size         [3]uint8
	Addr         [8]BlockNum
	AccessedTime uint32
	ModifiedTime uint32
}

const (
	FlagIsAllocated     = 0x8000 // Collides with S_IFREG
	FileTypeMask        = 0x6000
	FileTypeBlockDevice = 0x6000
	FileTypeDirectory   = 0x4000
	FileTypeCharDevice  = 0x2000
	FlagIsLargeFile     = 0x1000 // Collides with S_IFIFO
	FileTypePlainFile   = 0x0000
	FlagSetUID          = 0x0800 // S_ISUID
	FlagSetGID          = 0x0400 // S_ISGID
	// Unused: 0x0200, corresponds to S_ISVTX
	OwnerPermMask = 0x01c0 // S_IRWXU
	FlagOwnerR    = 0x0100 // S_IRUSR
	FlagOwnerW    = 0x0080 // S_IWUSR
	FlagOwnerX    = 0x0040 // S_IXUSR
	GroupPermMask = 0x0038 // S_IRWXG
	FlagGroupR    = 0x0020 // S_IRGRP
	FlagGroupW    = 0x0010 // S_IWGRP
	FlagGroupX    = 0x0008 // S_IXGRP
	OtherPermMask = 0x0007 // S_IRWXO
	FlagOtherR    = 0x0004 // S_IROTH
	FlagOtherW    = 0x0002 // S_IWOTH
	FlagOtherX    = 0x0001 // S_IXOTH
)

func ConvertFSFlagsToStandard(flags uint16) (os.FileMode, error) {
	// Preserve the permissions flags and discard the rest; we'll add them back
	// in shortly.
	mode := os.FileMode(flags) & os.ModePerm

	switch flags & FileTypeMask {
	case FileTypeBlockDevice:
		mode |= os.ModeDevice
	case FileTypeCharDevice:
		mode |= os.ModeCharDevice
	case FileTypeDirectory:
		mode |= os.ModeDir
	}

	if flags&FlagSetGID != 0 {
		mode |= os.ModeSetgid
	}

	if flags&FlagSetUID != 0 {
		mode |= os.ModeSetuid
	}

	return mode, nil
}

func ConvertStandardFlagsToFS(flags os.FileMode) (uint16, error) {
	mode := uint16(flags&os.ModePerm) | FlagIsAllocated

	switch flags & os.ModeType {
	case os.ModeCharDevice:
		mode |= FileTypeCharDevice
	case os.ModeDevice:
		mode |= FileTypeBlockDevice
	case os.ModeDir:
		mode |= FileTypeDirectory
	case 0:
		// Regular file, do nothing
	default:
		return mode,
			fmt.Errorf(
				"invalid mode flags %#.08x: type %#.08x is unsupported",
				flags,
				flags&os.ModeType,
			)
	}

	if flags&os.ModeSetgid != 0 {
		mode |= FlagSetGID
	}

	if flags&os.ModeSetuid != 0 {
		mode |= FlagSetUID
	}

	return mode, nil
}

func RawInodeToStat(inumber Inumber, inode RawInode) (disko.FileStat, error) {
	mode, err := ConvertFSFlagsToStandard(inode.Flags)
	if err != nil {
		return disko.FileStat{}, err
	}

	size := uint(inode.Size[0]) |
		(uint(inode.Size[1]) << 8) |
		(uint(inode.Size[2]) << 16)

	blocks := size / 512
	if size%512 != 0 {
		blocks++
	}

	return disko.FileStat{
		InodeNumber:  uint64(inumber),
		Nlinks:       uint64(inode.NLink),
		ModeFlags:    mode,
		Uid:          uint32(inode.UID),
		Gid:          uint32(inode.GID),
		Rdev:         0,
		Size:         int64(size),
		BlockSize:    512,
		NumBlocks:    int64(blocks),
		LastAccessed: time.Unix(int64(inode.AccessedTime), 0),
		LastModified: time.Unix(int64(inode.ModifiedTime), 0),
	}, nil
}

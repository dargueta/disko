package unixv6

import (
	"syscall"
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

func RawInodeToStat(inumber Inumber, inode RawInode) (disko.FileStat, error) {
	mode := uint32(inode.Flags)

	// Clear the IsAllocated flag because it corresponds to modern S_IFREG, and
	// also clear FlagIsLargeFile because it corresponds to S_IFIFO. These two
	// bits have no meaning outside the UV6FS implementation internals, so they
	// can be cleared.
	mode &^= FlagIsAllocated | FlagIsLargeFile

	// UnixV6 indicates a regular file with X & FileTypeMask = 0. Nowadays we
	// need to OR in the S_IFREG flag to indicate this is a regular file.
	if mode&syscall.S_IFMT == 0 {
		mode |= syscall.S_IFREG
	}

	return disko.FileStat{
		InodeNumber: uint64(inumber),
		Nlinks:      uint64(inode.NLink),
		ModeFlags:   mode,
		Uid:         uint32(inode.UID),
		Gid:         uint32(inode.GID),
		Rdev:        0,
		// TODO size
		BlockSize: 512,
		// TODO blocks
		LastAccessed: time.Unix(int64(inode.AccessedTime), 0),
		LastModified: time.Unix(int64(inode.ModifiedTime), 0),
	}, nil
}

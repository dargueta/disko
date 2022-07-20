package disko

////////////////////////////////////////////////////////////////////////////////
// File attribute flags

const (
	S_IXOTH = 1 << iota
	S_IWOTH = 1 << iota
	S_IROTH = 1 << iota
	S_IXGRP = 1 << iota
	S_IWGRP = 1 << iota
	S_IRGRP = 1 << iota
	S_IXUSR = 1 << iota
	S_IWUSR = 1 << iota
	S_IRUSR = 1 << iota
	S_ISVTX = 1 << iota
	S_ISGID = 1 << iota
	S_ISUID = 1 << iota
	S_IFIFO = 1 << iota
	S_IFCHR = 1 << iota
	S_IFDIR = 1 << iota
	S_IFREG = 1 << iota
)

const S_IEXEC = S_IXUSR
const S_IWRITE = S_IWUSR
const S_IREAD = S_IRUSR

const S_IFBLK = 0x6000  // 0110 0000 0000 0000
const S_IFLNK = 0xa000  // 1010 0000 0000 0000
const S_IFSOCK = 0xc000 // 1100 0000 0000 0000
const S_IFMT = 0xf000

const S_IRWXO = S_IXOTH | S_IWOTH | S_IROTH
const S_IRWXG = S_IXGRP | S_IWGRP | S_IRGRP
const S_IRWXU = S_IXUSR | S_IWUSR | S_IRUSR

////////////////////////////////////////////////////////////////////////////////
// Mount flags
// Descriptions taken from https://man7.org/linux/man-pages/man2/mount.2.html

// MS_RDONLY means: mount the file system as read-only.
const MS_RDONLY = 0x00000001
const MS_SYNCHRONOUS = 0x00000010
const MS_REMOUNT = 0x00000020
const MS_MANDLOCK = 0x00000040
const MS_DIRSYNC = 0x00000080
const MS_NOSYMFOLLOW = 0x00000100
const MS_NOATIME = 0x00000400
const MS_NODIRATIME = 0x00000800
const MS_RELATIME = 0x00200000
const MS_I_VERSION = 0x00800000
const MS_STRICTATIME = 0x01000000
const MS_LAZYTIME = 0x02000000
const MS_MGC_MSK = 0xffff0000
const MS_MGC_VAL = 0xc0ed0000

const DEFAULT_MOUNT_FLAGS = MS_MGC_VAL | MS_REMOUNT

const O_RDONLY = 0x00000000
const O_WRONLY = 0x00000001
const O_RDWR = 0x00000002
const O_APPEND = 0x00000008
const O_CREATE = 0x00000200
const O_TRUNC = 0x00000400
const O_EXCL = 0x00000800
const O_SYNC = 0x00002000
const O_NOFOLLOW = 0x00100000
const O_DIRECTORY = 0x00200000
const O_TMPFILE = 0x00800000
const O_NOATIME = 0x01000000
const O_PATH = 0x02000000

const O_ACCMODE = O_RDONLY | O_RDWR | O_WRONLY

// const O_TEXTORBINMODE = O_TEXT | O_BINARY

////////////////////////////////////////////////////////////////////////////////

type IOFlags int

func (flags IOFlags) Append() bool {
	return int(flags)&O_APPEND != 0
}

func (flags IOFlags) CanRead() bool {
	masked := int(flags) & O_ACCMODE
	return masked == O_RDWR || masked == O_WRONLY
}

func (flags IOFlags) CanWrite() bool {
	masked := int(flags) & O_ACCMODE
	return masked == O_RDWR || masked == O_RDONLY
}

func (flags IOFlags) Create() bool {
	return int(flags)&O_CREATE != 0
}

func (flags IOFlags) Truncate() bool {
	return int(flags)&O_TRUNC != 0
}

func (flags IOFlags) Exclusive() bool {
	return int(flags)&O_EXCL != 0
}

func (flags IOFlags) Synchronous() bool {
	return int(flags)&O_SYNC != 0
}

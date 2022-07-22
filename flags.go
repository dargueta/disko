package disko

import "os"

////////////////////////////////////////////////////////////////////////////////
// File attribute flags

const (
	S_IXOTH = os.FileMode(1 << iota)
	S_IWOTH = os.FileMode(1 << iota)
	S_IROTH = os.FileMode(1 << iota)
	S_IXGRP = os.FileMode(1 << iota)
	S_IWGRP = os.FileMode(1 << iota)
	S_IRGRP = os.FileMode(1 << iota)
	S_IXUSR = os.FileMode(1 << iota)
	S_IWUSR = os.FileMode(1 << iota)
	S_IRUSR = os.FileMode(1 << iota)
	S_ISVTX = os.FileMode(1 << iota)
	S_ISGID = os.FileMode(1 << iota)
	S_ISUID = os.FileMode(1 << iota)
	S_IFIFO = os.FileMode(1 << iota)
	S_IFCHR = os.FileMode(1 << iota)
	S_IFDIR = os.FileMode(1 << iota)
	S_IFREG = os.FileMode(1 << iota)
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

const DefaultFileModeFlags = S_IRUSR | S_IWUSR | S_IRGRP | S_IROTH
const DefaultDirModeFlags = os.ModeDir | S_IRWXU | S_IXGRP | S_IRGRP | S_IXOTH | S_IROTH

////////////////////////////////////////////////////////////////////////////////

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
const O_TMPFILE = 0x00800000 // Probably don't need this
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

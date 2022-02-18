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

// MS_NOSUID means: Do not honor set-user-ID and set-group-ID bits or file
// capabilities when executing programs from this filesystem.
//
// This is irrelevant, but the flag is included for the sake of completeness
// (also I copied and pasted this from my system's header file and don't want to
// change anything.)
const MS_NOSUID = 0x00000002

// MS_NODEV means: Do not allow access to devices (special files) on this
// filesystem.
//
// This is irrelevant, but the flag is included for the sake of completeness
// (also I copied and pasted this from my system's header file and don't want to
// change anything.)
const MS_NODEV = 0x00000004

// MS_NOEXEC means: Do not allow programs to be executed from this filesystem.
//
// This is irrelevant, but the flag is included for the sake of completeness
// (also I copied and pasted this from my system's header file and don't want to
// change anything.)
const MS_NOEXEC = 0x00000008
const MS_SYNCHRONOUS = 0x00000010
const MS_REMOUNT = 0x00000020
const MS_MANDLOCK = 0x00000040
const MS_DIRSYNC = 0x00000080
const MS_NOSYMFOLLOW = 0x00000100
const MS_NOATIME = 0x00000400
const MS_NODIRATIME = 0x00000800
const MS_BIND = 0x00001000
const MS_MOVE = 0x00002000
const MS_REC = 0x00004000
const MS_SILENT = 0x00008000
const MS_POSIXACL = 0x00010000
const MS_UNBINDABLE = 0x00020000
const MS_PRIVATE = 0x00040000
const MS_SLAVE = 0x00080000
const MS_SHARED = 0x00100000
const MS_RELATIME = 0x00200000
const MS_KERNMOUNT = 0x00400000
const MS_I_VERSION = 0x00800000
const MS_STRICTATIME = 0x01000000
const MS_LAZYTIME = 0x02000000
const MS_RMT_MASK = 0x02800051
const MS_SUBMOUNT = 0x04000000
const MS_NOREMOTELOCK = 0x08000000
const MS_NOSEC = 0x10000000
const MS_BORN = 0x20000000
const MS_ACTIVE = 0x40000000
const MS_MGC_MSK = 0xffff0000
const MS_MGC_VAL = 0xc0ed0000

const DEFAULT_MOUNT_FLAGS = MS_NOSUID | MS_NODEV | MS_NOEXEC | MS_REMOUNT

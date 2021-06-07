package disko

const (
	S_IXOTH = 1 << iota // 00001
	S_IWOTH = 1 << iota // 00002
	S_IROTH = 1 << iota
	S_IXGRP = 1 << iota
	S_IWGRP = 1 << iota // 00010
	S_IRGRP = 1 << iota
	S_IXUSR = 1 << iota
	S_IWUSR = 1 << iota
	S_IRUSR = 1 << iota // 00100
	S_ISVTX = 1 << iota
	S_ISGID = 1 << iota
	S_ISUID = 1 << iota
	S_IFIFO = 1 << iota // 01000
	S_IFCHR = 1 << iota // 02000
	S_IFDIR = 1 << iota // 04000
	S_IFREG = 1 << iota // 08000
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

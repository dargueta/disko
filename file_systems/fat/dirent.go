package fat

import (
	"encoding/binary"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/errors"
)

// fatEpoch is 1980-01-01 00:00:00 at local time.
var fatEpoch = time.Unix(315561600, 0)

const (
	// AttrReadOnly is an attribute flag marking a directory entry as read-only.
	AttrReadOnly = 1

	// AttrHidden is an attribute flag marking a directory entry as "hidden", meaning it
	// wouldn't show up in normal directory listings. This is most commonly used for
	// hiding operating system files from normal users.
	//
	// Drivers don't need to honor this flag when reading, but should not modify it unless
	// explicitly requested by the user.
	AttrHidden = 2

	// AttrSystem is an attribute flag marking a directory entry as essential to the
	// operating system and must not be moved (e.g. during defragmentation) because the
	// OS may have hard-coded pointers to the file.
	AttrSystem = 4

	// AttrVolumeLabel is an attribute flag that marks a file as containing the true
	// volume label of the file system. It must reside in the root directory, and there
	// must be only one. For compatibility reasons it should be the first directory entry
	// after `.` and `..` but this is not required.
	//
	// The struct in the boot sector only has eleven bytes of space for the volume label.
	// This is not always enough, especially for systems or languages using multi-byte
	// character encodings.
	AttrVolumeLabel = 8

	// AttrDirectory is an attribute flag marking a directory entry as being a directory.
	AttrDirectory = 16

	// AttrArchived is an attribute flag used by some systems to mark a directory entry
	// as "dirty", and is set it whenever the directory entry is created or modified.
	// Archiving tools use this flag to determine whether the file/directory needs to be
	// backed up or not.
	AttrArchived = 32

	// AttrDevice is an attribute flag marking a directory entry as abstracting a device.
	// This is typically only found on in-memory file systems; if encountered on a disk,
	// it must not be modified.
	AttrDevice = 64

	// AttrReserved is an attribute flag that is undefined by the FAT standard and must
	// not be moified by tools.
	AttrReserved = 128
)

// RawDirent is the on-disk representation of a directory entry, broken down into its
// constituent fields.
type RawDirent struct {
	Name              [8]byte
	Extension         [3]byte
	AttributeFlags    uint8
	NTReserved        uint8
	CreatedTimeMillis uint8
	CreatedTime       uint16
	CreatedDate       uint16
	LastAccessedDate  uint16
	FirstClusterHigh  uint16
	LastModifiedTime  uint16
	LastModifiedDate  uint16
	FirstClusterLow   uint16
	FileSize          uint32
}

// Dirent is a representation of a FAT directory entry's data in a user-friendly format,
// e.g. 0x50FC is converted to a time.Time representing 2020-07-28 00:00:00 local time.
type Dirent struct {
	disko.DirectoryEntry
	os.FileInfo
	name           string
	AttributeFlags int
	NTReserved     int
	FirstCluster   ClusterID
	isDeleted      bool
	size           int64
	mode           os.FileMode
	stat           disko.FileStat
}

// GetLastAccessedAt returns the timestamp at which the directory entry was last accessed.
func (d *Dirent) GetLastAccessedAt() time.Time {
	return d.stat.LastAccessed
}

// SetLastAccessedAt sets the timestamp at which the directory entry was last accessed.
// It is an error to try to set this time before 1980-01-01 00:00:00 local time.
func (d *Dirent) SetLastAccessedAt(t time.Time) error {
	if t.Before(fatEpoch) {
		return errors.ErrArgumentOutOfRange
	}
	d.stat.LastAccessed = t
	return nil
}

func (d *Dirent) GetLastModifiedAt() time.Time {
	return d.stat.LastModified
}

// SetLastModifiedAt sets the timestamp at which the directory entry was last modified.
// It is an error to try to set this time before 1980-01-01 00:00:00 local time.
func (d *Dirent) SetLastModifiedAt(t time.Time) error {
	if t.Before(fatEpoch) {
		return errors.ErrArgumentOutOfRange
	}
	d.stat.LastModified = t
	return nil
}

// GetCreatedAt returns the timestamp at which the directory entry was created.
// It is an error to get this timestamp for a dirent that has been deleted.
func (d *Dirent) GetCreatedAt() (time.Time, error) {
	if d.isDeleted {
		return time.Unix(0, 0), errors.ErrNotFound
	}
	return d.stat.CreatedAt, nil
}

// SetCreatedAt sets the timestamp at which the directory entry was created.
// It is an error to try to set this time before 1980-01-01 00:00:00 local time, or to set
// this timestamp for a dirent that has been deleted.
func (d *Dirent) SetCreatedAt(t time.Time) error {
	if t.Before(fatEpoch) {
		return errors.ErrArgumentOutOfRange
	} else if d.isDeleted {
		return errors.ErrNotFound
	}

	d.stat.CreatedAt = t
	return nil
}

// GetDeletedAt returns the time at which the directory entry was deleted, or the Unix
// epoch and an error object if the file hasn't been deleted. Note that the Unix epoch,
// 1970-01-01 00:00:00 UTC, is a decade *before* the earliest representable date in a
// FAT timestamp, 1980-01-01 00:00:00 local time.
func (d *Dirent) GetDeletedAt() (time.Time, error) {
	if !d.isDeleted {
		return disko.UndefinedTimestamp, errors.ErrInvalidArgument
	}
	return d.stat.DeletedAt, nil
}

// SetDeletedAt sets the time at which a directory entry was deleted, marking it as deleted
// if it hasn't already been.
// It is an error to try to set this time before 1980-01-01 00:00:00 local time.
func (d *Dirent) SetDeletedAt(t time.Time) error {
	if t.Before(fatEpoch) {
		return errors.ErrArgumentOutOfRange
	}
	d.isDeleted = true
	return d.SetCreatedAt(t)
}

// DirentSize is the size of a single raw directory entry, in bytes.
const DirentSize = 32

// DateFromInt converts the FAT on-disk representation of a date into a Go time.Time
// object.
func DateFromInt(value uint16) time.Time {
	createDay := int(value & 0x001f)
	createMonth := time.Month((value >> 5) & 0x000f)
	createYear := int(1980 + (value >> 9))

	return time.Date(createYear, createMonth, createDay, 0, 0, 0, 0, nil)
}

// TimestampFromParts converts a FAT timestamp into a time.Time object. datePart is
// required; timePart and hundredths should be 0 if they're not present in the source
// field(s).
func TimestampFromParts(datePart uint16, timePart uint16, hundredths uint8) time.Time {
	dateDt := DateFromInt(datePart)

	seconds := int((timePart & 0x001f) * 2)
	if hundredths >= 100 {
		seconds += 1
		hundredths -= 100
	}

	minutes := int((timePart >> 5) & 0x003f)
	hours := int(timePart >> 11)
	nanoseconds := int(timePart * 10000)

	return time.Date(
		dateDt.Year(), dateDt.Month(), dateDt.Day(), hours, minutes, seconds, nanoseconds, nil)
}

// AttrFlagsToFileMode converts FAT attribute flags into the mode flags used by
// syscall.Stat_t.Mode.
func AttrFlagsToFileMode(flags uint8) os.FileMode {
	var mode os.FileMode

	// FAT has no way to mark files as executable or not, so the executable bit is always set.
	if (flags & AttrReadOnly) != 0 {
		mode = 0o755
	} else {
		mode = 0o777
	}

	if (flags & AttrDirectory) != 0 {
		mode |= syscall.S_IFDIR
	} else if (flags & AttrDevice) != 0 {
		mode |= syscall.S_IFCHR
	} else {
		mode |= syscall.S_IFREG
	}

	return mode
}

// NewRawDirentFromBytes deserializes 32 bytes into a RawDirent struct for further
// processing.
func NewRawDirentFromBytes(data []byte) (RawDirent, error) {
	dirent := RawDirent{
		AttributeFlags:    data[12],
		NTReserved:        data[13],
		CreatedTimeMillis: data[14],
		CreatedTime:       binary.LittleEndian.Uint16(data[15:17]),
		CreatedDate:       binary.LittleEndian.Uint16(data[17:19]),
		LastAccessedDate:  binary.LittleEndian.Uint16(data[19:21]),
		FirstClusterHigh:  binary.LittleEndian.Uint16(data[21:23]),
		LastModifiedTime:  binary.LittleEndian.Uint16(data[23:25]),
		LastModifiedDate:  binary.LittleEndian.Uint16(data[25:27]),
		FirstClusterLow:   binary.LittleEndian.Uint16(data[27:29]),
		FileSize:          binary.LittleEndian.Uint32(data[29:32]),
	}

	copy(dirent.Name[:], data[:8])
	copy(dirent.Extension[:], data[8:11])
	return dirent, nil
}

func TimeToTimespec(t time.Time) syscall.Timespec {
	return syscall.NsecToTimespec(t.UnixNano())
}

// NewDirentFromRaw creates a fully processed Dirent from a raw one, such as converting
// 24-bit values into time.Time values.
func NewDirentFromRaw(bootSector *FATBootSector, rawDirent *RawDirent) (Dirent, error) {
	lastModified := TimestampFromParts(
		rawDirent.LastModifiedDate, rawDirent.LastModifiedTime, 0)
	size := int64(rawDirent.FileSize)
	sizeInClusters := size / int64(bootSector.BytesPerCluster)
	if size%int64(bootSector.BytesPerCluster) != 0 {
		sizeInClusters++
	}

	mode := AttrFlagsToFileMode(rawDirent.AttributeFlags)
	firstCluster := (uint32(rawDirent.FirstClusterHigh) << 16) | uint32(rawDirent.FirstClusterLow)

	isDeleted := rawDirent.Name[0] == 0xE5

	var createdAt time.Time
	if isDeleted {
		createdAt = time.Unix(0, 0)
	} else {
		createdAt = TimestampFromParts(
			rawDirent.CreatedDate, rawDirent.CreatedTime, rawDirent.CreatedTimeMillis)
	}

	dirent := Dirent{
		stat: disko.FileStat{
			DeviceID: 0,
			// FAT systems have no concept of inodes but a quick way to see if two
			// directory entries point to the same thing is to see if the first cluster
			// is the same. Thus, we'll sorta cheat and use that as a file ID.
			InodeNumber:  uint64(firstCluster),
			Nlinks:       1,
			ModeFlags:    mode,
			Uid:          0,
			Gid:          0,
			Rdev:         0,
			Size:         size,
			BlockSize:    int64(bootSector.BytesPerCluster),
			NumBlocks:    sizeInClusters,
			LastAccessed: DateFromInt(rawDirent.LastAccessedDate),
			LastModified: lastModified,
			CreatedAt:    createdAt,
		},
		AttributeFlags: int(rawDirent.AttributeFlags),
		NTReserved:     int(rawDirent.NTReserved),
		isDeleted:      isDeleted,
		size:           size,
		mode:           os.FileMode(mode),
		FirstCluster:   ClusterID(firstCluster),
	}

	trimmedName := strings.TrimRight(string(rawDirent.Name[:]), " ")
	trimmedExt := strings.TrimRight(string(rawDirent.Extension[:]), " ")

	if trimmedName[0] == 0xE5 {
		// Represents a deleted file, and the real first character of the filename is in
		// CreatedTimeMillis
		trimmedName = string([]byte{rawDirent.CreatedTimeMillis}) + trimmedName[1:]
	} else if trimmedName[0] == 0x05 {
		// First character of the filename is E5
		trimmedName = "\xe5" + trimmedName[1:]
	} else if trimmedName[0] == 0 {
		// This directory entry is free and thus invalid.
		return Dirent{}, errors.ErrNotFound
	}

	if trimmedExt == "" {
		dirent.name = trimmedName
	} else {
		dirent.name = trimmedName + "." + trimmedExt
	}

	return dirent, nil
}

// clusterToDirentSlice processes a slice of bytes the size of a full cluster into a slice
// of directory entries.
func (drv *FATDriver) clusterToDirentSlice(data []byte) ([]Dirent, error) {
	allDirents := []Dirent{}
	bootSector := drv.fs.GetBootSector()

	for i := 0; i < bootSector.DirentsPerCluster; i++ {
		offset := i * DirentSize
		rawDirent, _ := NewRawDirentFromBytes(data[offset : offset+DirentSize])

		dirent, err := NewDirentFromRaw(bootSector, &rawDirent)
		if err != nil {
			// If this is a DriverError there may be further action we can take.
			drverr, ok := err.(errors.DriverError)
			if !ok {
				// Not a DriverError, nothing else we can do.
				return nil, err
			}

			// If the error code is ENOENT then that means this directory entry is free
			// and we've hit the end of the directory.
			if errors.ErrNotFound.IsSameError(drverr) {
				break
			}
			// Else: We failed for a different reason. Pass this error up to the
			// caller.
			return nil, err
		}
		// Else: Success!
		allDirents = append(allDirents, dirent)
	}

	return allDirents, nil
}

// Dirent implementation of FileInfo -------------------------------------------

// Name returns the name of the directory entry.
//
// TODO (dargueta): Implement LFN support.
func (d Dirent) Name() string { return d.name }

// Size is the size of the directory entry if and ONLY if it's a regular file.
//
// Directories will have this value set to 0. The only way to tell the size of a directory
// is to recurse through it completely, and that's kinda excessive.
//
// TODO (dargueta): Is there a more efficient way to get the size for directories?
// All directories must contain at least `.` and `..` entries, so they'll always be at
// least 64 bytes.
func (d Dirent) Size() int64 { return d.size }

// Mode returns the mode flags of this directory entry.
func (d Dirent) Mode() os.FileMode { return d.mode }

func (d Dirent) ModTime() time.Time { return d.GetLastModifiedAt() }

func (d Dirent) IsDir() bool { return d.mode.IsDir() }

func (d Dirent) Sys() interface{} { return d.stat }

func (d Dirent) Stat() disko.FileStat { return d.stat }

// -----------------------------------------------------------------------------

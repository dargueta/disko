// Package lbr implements access to LU library files.
//
// It currently doesn't support the compressed formats with extensions .LQR (using
// the CP/M version of "SQUEEZE") or .LZR (using "CRUNCH"). I need to find
// definitions of those formats first.
//
// The current implementation only supports version 5 of the LBR specification

package lbr

import (
	"encoding/binary"
	"io"
	"strings"
	"time"

	"github.com/dargueta/disko"
)

// This is December 31st, 1977. Since 0 is used as an invalid value, 1978-01-01
// is the earliest representable date.
var lbrEpoch = time.Unix(52403200, 0)

// RawDirent is the on-disk representation of a directory entry.
//
// Note: All multibyte values are stored in little-endian order, with the least
// significant bytes first.
type RawDirent struct {
	// Status indicates how the record should be interpreted.
	//
	// Three values are defined: 0x00 indicates the record is in use; 0xFE marks
	// a deleted entry, and 0xFF marks an unused entry that can be reused for a
	// new file. Any other value is undefined and should be treated as an unused
	// entry.
	Status uint8
	// Name is the name of the file, right-padded with spaces. The directory must
	// must have this field set to entirely spaces, and only the directory can
	// have a blank Name.
	Name [8]byte
	// Extension is the extension of the file, right-padded with spaces. If not
	// used, it must be all spaces. Like Name, this must be set to all spaces
	// for the directory. (Files are allowed to have no extension, however.)
	Extension [3]byte
	// Index is the offset of the data for this entry from the beginning of the
	// file, in increments of 128 bytes. This is always 0 for the directory.
	Index uint16
	// SizeInSectors is the length of the data for this entry, in sectors.
	SizeInSectors uint16
	// CRCValue is the computed CRC of the data, using the CCITT algorithm. Pad
	// bytes for "short" sectors *are* included in the computation.
	CRCValue uint16
	// CreatedDate is the date the file was created, represented as the number
	// of days since December 31st, 1977. 0 is used as a null value, so the
	// earliest representable date is January 1st, 1978.
	//
	// If used in the directory entry, this is the date of creation of the
	// library file.
	CreatedDate uint16
	// LastModifiedDate is the date the file was last modified. It uses the same
	// representation as CreatedDate.
	//
	// At the library level, the directory is changed by adding, deleting, or
	// renaming members, or by reorganizing/compacting the library.
	LastModifiedDate uint16
	// CreatedTime is the time portion of the file's creation timestamp. It's
	// stored as a 16-bit number with three fields as shown below:
	//
	//     15      8        0
	// 		hhhhhmmm|mmmsssss
	//
	// There are only five bits for the seconds, because the time has a maximum
	// resolution of two seconds. You'll need to multiply this value by 2 to get
	// the actual seconds portion.
	//
	// The specification this is built from doesn't say whether the time is
	// local or UTC. Given the age of the format, this implementation assumes
	// that all timestamps are local time.
	CreatedTime uint16
	// LastModifiedTime is time portion of the file's last modified timestamp.
	// See CreatedTime for its format and other notes.
	LastModifiedTime uint16
	// PadCount is the number of unused bytes at the end of the last sector of
	// the file's data, if applicable. Valid values are 0-127, inclusive. This
	// is always 0 for the directory.
	PadCount uint8
	// Filler is unused padding bytes. It must be set to nulls when writing, and
	// ignored when reading.
	Filler [5]uint8
}

// LBRDirent is a directory entry for LBR files.
type LBRDirent struct {
	disko.DirectoryEntry
	name string
}

// MakeTimestampFromParts creates a Go time.Time object from the date and time
// portions of a time in an LBR archive.
func MakeTimestampFromParts(datePart uint16, timePart uint16) time.Time {
	if datePart == 0 {
		return lbrEpoch
	}

	seconds := int(timePart&0x1f) * 2
	minutes := int((timePart >> 5) & 0x3f)
	hours := int(timePart >> 11)
	date := lbrEpoch.AddDate(0, 0, int(datePart))

	return time.Date(date.Year(), date.Month(), date.Day(), hours, minutes, seconds, 0, nil)
}

// NewDirent creates a directory entry from the next bytes in the stream.
func NewDirent(stream *io.Reader) (LBRDirent, error) {
	raw := RawDirent{}
	err := binary.Read(*stream, binary.LittleEndian, &raw)
	if err != nil {
		return LBRDirent{}, err
	}

	createdTimestamp := MakeTimestampFromParts(raw.CreatedDate, raw.CreatedTime)
	lastModifiedTimestamp := MakeTimestampFromParts(raw.LastModifiedDate, raw.LastModifiedTime)

	stem := strings.TrimRight(string(raw.Name[:]), " ")
	extension := strings.TrimRight(string(raw.Extension[:]), " ")
	var fullName string

	if extension != "" {
		fullName = stem + "." + extension
	} else {
		fullName = stem
	}

	newEntry := LBRDirent{
		DirectoryEntry: disko.DirectoryEntry{
			Stat: disko.FileStat{
				Nlink:        1,
				Mode:         0o777,
				Uid:          0,
				Gid:          0,
				Size:         int64(raw.SizeInSectors)*128 - int64(raw.PadCount),
				Blksize:      128,
				Blocks:       int64(raw.SizeInSectors),
				LastModified: lastModifiedTimestamp,
				CreatedAt:    createdTimestamp,
			},
		},
		name: fullName,
	}
	return newEntry, nil
}

// Name returns the name of the directory entry.
func (dirent *LBRDirent) Name() string {
	return dirent.name
}

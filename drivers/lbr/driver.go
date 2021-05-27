// Package lbr implements access to LU library files.
//
// It currently doesn't support the compressed formats with extensions .LQR (using
// the CP/M version of "SQUEEZE") or .LZR (using "CRUNCH"). I need to find definitions
// of those formats first.

package lbr

import (
	"strings"
	"syscall"
	"time"

	"github.com/dargueta/disko"
)

// This is December 31st, 1977. Since 0 is used as an invalid value, 1978-01-01
// is the earliest representable date.
var lbrEpoch = time.Unix(52403200, 0)

type RawDirent struct {
	Status    uint8
	Name      [8]byte
	Extension [3]byte
	// Index is the offset of the data for this entry from the beginning of the
	// file, in increments of 128 bytes.
	Index uint16
	// SizeInSectors is the length of the data for this entry, in sectors.
	SizeInSectors uint16
	// CRCValue is the computed CRC of the data, using the CCITT algorithm. Pad
	// bytes for "short" sectors *are* included in the computation.
	CRCValue         uint16
	CreatedDate      uint16
	LastModifiedDate uint16
	CreatedTime      uint16
	LastModifiedTime uint16
	PadCount         uint8
	Filler           [5]uint8
}

type LBRDirent struct {
	disko.DirectoryEntry
	name string
}

func NewTimeFromPieces(datePart uint16, timePart uint16) time.Time {
	if datePart == 0 {
		return time.Unix(0, 0)
	}

	seconds := int(timePart&0x1f) * 2
	minutes := int((timePart >> 5) & 0x3f)
	hours := int(timePart >> 11)
	date := lbrEpoch.AddDate(0, 0, int(datePart))

	return time.Date(date.Year(), date.Month(), date.Day(), hours, minutes, seconds, 0, nil)
}

func NewDirentFromRawDirent(raw *RawDirent) (LBRDirent, error) {
	createdTimestamp := NewTimeFromPieces(raw.CreatedDate, raw.CreatedTime)
	lastModifiedTimestamp := NewTimeFromPieces(raw.LastModifiedDate, raw.LastModifiedTime)

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
			Stat: syscall.Stat_t{
				Nlink:   1,
				Mode:    0o777,
				Uid:     0,
				Gid:     0,
				Size:    int64(raw.SizeInSectors)*128 - int64(raw.PadCount),
				Blksize: 128,
				Blocks:  int64(raw.SizeInSectors),
				Mtim:    syscall.Timespec{Sec: lastModifiedTimestamp.Unix()},
				Ctim:    syscall.Timespec{Sec: createdTimestamp.Unix()},
			},
		},
		name: fullName,
	}
	return newEntry, nil
}

func (dirent *LBRDirent) Name() string {
	return dirent.name
}

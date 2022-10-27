// Package fat implements a driver for accessing FAT file systems.
package fat

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/dargueta/disko"
)

type SectorID uint32
type ClusterID uint32

// RawFATBootSectorWithBPB is the on-disk representation of the boot sector.
//
// Note: This is only the section of the boot sector common to all FAT versions. Other
// fields specific to a particular version can be found in RawFAT12BootSector, RawFAT16BootSector,
// and RawFAT32BootSector.
type RawFATBootSectorWithBPB struct {
	JmpBoot           [3]byte
	OEMName           [8]byte
	BytesPerSector    uint16
	SectorsPerCluster uint8
	ReservedSectors   uint16
	NumFATs           uint8
	RootEntryCount    uint16
	totalSectors16    uint16
	Media             uint8
	sectorsPerFAT16   uint16
	SectorsPerTrack   uint16
	NumHeads          uint16
	HiddenSectors     uint32
	totalSectors32    uint32
}

// FATBootSector extends RawFATBootSectorWithBPB with precomputed fields useful in other
// operations.
type FATBootSector struct {
	RawFATBootSectorWithBPB
	SectorsPerFAT     uint
	TotalFATSectors   uint
	RootDirSectors    uint
	BytesPerCluster   uint
	TotalClusters     uint
	TotalDataSectors  uint
	FirstDataSector   SectorID
	FATVersion        int
	DirentsPerCluster int
}

// DetermineFATVersion determines the version of the FAT file system based on the number
// of clusters on the system. (This is the only proper way to do so.)
func DetermineFATVersion(totalClusters uint) int {
	// These cluster counts, while odd-looking, are correct. They're taken directly from
	// Microsoft's FAT documentation, v1.03, page 14.
	if totalClusters < 4085 {
		return 12
	}
	if totalClusters < 65525 {
		return 16
	}
	return 32
}

// NewFATBootSectorFromStream reads the first 40 bytes of a disk image and returns a
// structure with detailed information on the file system.
//
// If an error occurs, it returns nil and an error object. There are no guarantees on
// the position of stream pointer in this case.
func NewFATBootSectorFromStream(reader io.Reader) (*FATBootSector, error) {
	rawHeader := RawFATBootSectorWithBPB{}

	err := binary.Read(reader, binary.LittleEndian, &rawHeader)
	if err != nil {
		return nil, disko.ErrIOFailed.WrapError(err)
	}

	var sectorsPerFAT32 uint32
	err = binary.Read(reader, binary.LittleEndian, &sectorsPerFAT32)
	if err != nil {
		return nil, disko.ErrIOFailed.WrapError(err)
	}

	var sectorsPerFAT uint
	if rawHeader.sectorsPerFAT16 != 0 {
		sectorsPerFAT = uint(rawHeader.sectorsPerFAT16)
	} else {
		sectorsPerFAT = uint(sectorsPerFAT32)
	}

	var totalSectors uint
	if rawHeader.totalSectors16 != 0 {
		totalSectors = uint(rawHeader.totalSectors16)
	} else {
		totalSectors = uint(rawHeader.totalSectors32)
	}

	// The number of sectors taken up by the root directory. On FAT32 systems, this will
	// be 0.
	rootDirSectors := uint(
		((rawHeader.RootEntryCount * 32) + (rawHeader.BytesPerSector - 1)) / rawHeader.BytesPerSector)

	totalFATSectors := uint(rawHeader.NumFATs) * sectorsPerFAT
	dataSectors := totalSectors - uint(rawHeader.ReservedSectors) + totalFATSectors + uint(rootDirSectors)
	totalClusters := dataSectors / uint(rawHeader.SectorsPerCluster)

	// BytesPerSector must be 512, 1024, 2048, or 4096.
	switch rawHeader.BytesPerSector {
	case 512:
	case 1024:
	case 2048:
	case 4096:
	default:
		message := fmt.Sprintf(
			"corruption detected: BytesPerSector must be 512, 1024, 2048, or 4096, got %d",
			rawHeader.BytesPerSector)
		return nil, disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	// SectorsPerCluster must be 2^x with x in [0, 8)
	switch rawHeader.SectorsPerCluster {
	case 1:
	case 2:
	case 4:
	case 8:
	case 16:
	case 32:
	case 64:
	case 128:
	default:
		message := fmt.Sprintf(
			"corruption detected: SectorsPerCluster must be a power of 2 in 1-128, got %d",
			rawHeader.SectorsPerCluster)
		return nil, disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	fatVersion := DetermineFATVersion(totalClusters)
	if fatVersion == 32 && rootDirSectors != 0 {
		message := fmt.Sprintf(
			"corruption detected: RootDirectorySectors is nonzero for a FAT32 disk: %d",
			rootDirSectors)

		return nil, disko.ErrFileSystemCorrupted.WithMessage(message)

	}

	bytesPerCluster := uint(rawHeader.BytesPerSector) * uint(rawHeader.SectorsPerCluster)
	if bytesPerCluster > 32768 {
		message := fmt.Sprintf(
			"corruption detected: BytesPerCluster cannot exceed 32,768 but got %d",
			bytesPerCluster)

		return nil, disko.ErrFileSystemCorrupted.WithMessage(message)
	}

	processedHeader := FATBootSector{
		RawFATBootSectorWithBPB: RawFATBootSectorWithBPB{
			JmpBoot:           rawHeader.JmpBoot,
			OEMName:           rawHeader.OEMName,
			BytesPerSector:    rawHeader.BytesPerSector,
			SectorsPerCluster: rawHeader.SectorsPerCluster,
			ReservedSectors:   rawHeader.ReservedSectors,
			NumFATs:           rawHeader.NumFATs,
			RootEntryCount:    rawHeader.RootEntryCount,
			totalSectors16:    rawHeader.totalSectors16,
			Media:             rawHeader.Media,
			sectorsPerFAT16:   rawHeader.sectorsPerFAT16,
			SectorsPerTrack:   rawHeader.SectorsPerTrack,
			NumHeads:          rawHeader.NumHeads,
			HiddenSectors:     rawHeader.HiddenSectors,
			totalSectors32:    rawHeader.totalSectors32,
		},
		SectorsPerFAT:     sectorsPerFAT,
		TotalFATSectors:   totalFATSectors,
		RootDirSectors:    rootDirSectors,
		BytesPerCluster:   bytesPerCluster,
		TotalClusters:     totalClusters,
		TotalDataSectors:  totalSectors - (uint(rawHeader.ReservedSectors) + totalFATSectors + rootDirSectors),
		FirstDataSector:   SectorID(uint(rawHeader.ReservedSectors) + rootDirSectors),
		FATVersion:        fatVersion,
		DirentsPerCluster: int(bytesPerCluster) / DirentSize,
	}

	return &processedHeader, nil
}

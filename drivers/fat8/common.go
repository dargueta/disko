package fat8

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/dargueta/disko"
)

type Geometry struct {
	TotalTracks          uint
	TrueTotalTracks      uint
	TotalClusters        uint
	SectorsPerTrack      uint
	SectorsPerFAT        uint
	SectorsPerCluster    uint
	BytesPerCluster      uint
	DirectoryTrackNumber uint
	DirectoryTrackStart  PhysicalBlock
	InfoSectorStart      PhysicalBlock
	FATsStart            PhysicalBlock
}

func GetGeometry(totalBlocks uint) (Geometry, error) {
	var geo Geometry
	switch totalBlocks {
	case 640:
		geo.TotalTracks = 40
		geo.TrueTotalTracks = 40
		geo.SectorsPerTrack = 16
		geo.DirectoryTrackNumber = 18
	case 1898:
		geo.TotalTracks = 73
		geo.TrueTotalTracks = 77
		geo.SectorsPerTrack = 26
		geo.DirectoryTrackNumber = 35
	case 2002:
		// 2002 (77 tracks * 26 sectors/track) is an alias for 1898. This is
		// because, while the IBM 3070 disk technically has 77 tracks, only 73
		// of those can hold data.
		geo.TotalTracks = 73
		geo.TrueTotalTracks = 77
		geo.SectorsPerTrack = 26
		geo.DirectoryTrackNumber = 35
	default:
		return geo,
			fmt.Errorf("bad number of blocks; expected 2002, 1898, or 640, got %d", totalBlocks)
	}

	geo.TotalClusters = geo.TotalTracks * 2
	geo.SectorsPerCluster = geo.SectorsPerTrack / 2
	geo.BytesPerCluster = geo.SectorsPerCluster * 128
	geo.SectorsPerFAT = (geo.TotalClusters + (-geo.TotalClusters % 128)) / 128

	directoryTrackStart := (geo.DirectoryTrackNumber - 1) * geo.SectorsPerTrack
	geo.DirectoryTrackStart = PhysicalBlock(directoryTrackStart)
	geo.FATsStart = PhysicalBlock(directoryTrackStart + geo.SectorsPerTrack - (geo.SectorsPerFAT * 3))
	geo.InfoSectorStart = geo.FATsStart - 1

	return geo, nil
}

// FilenameToBytes converts a filename string to its on-disk representation. The
// returned name will be normalized to uppercase.
// TODO(dargueta): Ensure the filename has no invalid characters.
func FilenameToBytes(name string) ([]byte, error) {
	parts := strings.SplitN(name, ".", 2)

	// Unless we got an empty string, this will always have at least one element,
	// the stem of the filename. This cannot be longer than 6 characters.
	if len(parts[0]) > 6 {
		message := fmt.Sprintf(
			"filename stem can be at most six characters: `%s`", parts[0])
		return nil, disko.NewDriverErrorWithMessage(disko.ENAMETOOLONG, message)
	}

	var paddedName string
	// If there are two parts to the filename then there's an extension (probably)
	if len(parts) == 2 {
		if len(parts[1]) == 0 {
			// Second part after the period is empty, which means the filename
			// ended with a period. This is stupid, but not prohibited by the
			// standard (I think) so we must support it.
			parts[1] = "."
		} else if len(parts[1]) > 3 {
			// Extension is longer than three characters.
			message := fmt.Sprintf(
				"filename extension can be at most three characters: `%s`", parts[1])
			return nil, disko.NewDriverErrorWithMessage(disko.ENAMETOOLONG, message)
		}

		paddedName = fmt.Sprintf("%-6s%-3s", parts[0], parts[1])
	} else {
		// Filename has no extension.
		paddedName = fmt.Sprintf("%-9s", parts[0])
	}

	return []byte(strings.ToUpper(paddedName)), nil
}

// BytesToFilename converts the on-disk representation of a filename into its
// user-friendly form.
// TODO(dargueta): Validate binary data
func BytesToFilename(rawName []byte) (string, error) {
	stem := bytes.TrimRight(rawName[:6], " ")
	extension := bytes.TrimRight(rawName[6:], " ")

	var name string
	if len(extension) > 0 {
		name = string(stem) + "." + string(extension)
	} else {
		name = string(stem)
	}
	return strings.ToUpper(name), nil
}

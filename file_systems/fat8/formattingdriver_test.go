package fat8

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/dargueta/disko"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/empty-floppy.img
var emptyFloppyImage []byte

//go:embed testdata/empty-minifloppy.img
var emptyMinifloppyImage []byte

func FirstDifference(left, right []byte) int {
	if len(left) > len(right) {
		return len(left)
	} else if len(right) > len(left) {
		return len(right)
	}

	for i := 0; i < len(left); i++ {
		if left[i] != right[i] {
			return i
		}
	}
	return -1
}

func ValidateImage(t *testing.T, totalBlocks, sectorsPerTrack uint, expectedImage []byte) {
	totalBytes := totalBlocks * 128
	require.EqualValuesf(
		t,
		len(expectedImage),
		totalBytes,
		"embedded test data is wrong: image should be %d bytes, got %d",
		totalBytes,
		len(expectedImage))

	tmpFile, err := os.CreateTemp("", "")
	require.NoError(t, err, "failed to create temporary file")
	defer tmpFile.Close()

	driver := NewDriverFromFile(tmpFile)
	err = driver.FormatImage(
		disko.FSStat{
			BlockSize:   128,
			TotalBlocks: uint64(totalBlocks),
		},
	)
	require.NoError(t, err, "formatting the image failed")

	imageContents := make([]byte, totalBytes)
	bytesRead, err := tmpFile.ReadAt(imageContents, 0)
	if err != nil {
		require.ErrorIs(t, err, io.EOF, "failed to read the image file")
	}
	require.EqualValues(t, totalBytes, bytesRead, "image size is wrong")

	// Iterate sector by sector and show an error message for each sector that
	// differs from expected.
	for i := uint(0); i < totalBytes; i += 128 {
		currentSector := imageContents[i : i+128]
		expectedSector := imageContents[i : i+128]

		// Do a quick check to see if the sectors are equal. Don't bother with
		// the rest if they are.
		if bytes.Equal(currentSector, expectedSector) {
			continue
		}

		// The sectors differ, find the first differing byte within this sector.
		diffAt := uint(0)
		for ; diffAt < 128; diffAt++ {
			if currentSector[diffAt] != expectedSector[diffAt] {
				break
			}
		}

		absoluteSector := i / 128
		track := absoluteSector / sectorsPerTrack
		sectorInTrack := absoluteSector % sectorsPerTrack

		t.Errorf(
			"Images not equal in track %d, sector %d (absolute sector %d);"+
				" first differing byte at index %d (base 0); expected %#02x,"+
				" got %#02x",
			track+1,
			sectorInTrack+1,
			absoluteSector,
			diffAt,
			expectedSector[diffAt],
			currentSector[diffAt],
		)
	}
}

func TestFormattingFloppy(t *testing.T) {
	ValidateImage(t, 2002, 26, emptyFloppyImage)
}

func TestFormattingMiniFloppy(t *testing.T) {
	ValidateImage(t, 640, 16, emptyMinifloppyImage)
}

package fat8

import (
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/dargueta/disko"
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
	if len(expectedImage) != int(totalBytes) {
		t.Fatalf(
			"Embedded test data is wrong: image should be %d bytes, got %d",
			totalBytes,
			len(expectedImage),
		)
	}

	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	driver := NewDriverFromFile(tmpFile)
	err = driver.Format(disko.FSStat{TotalBlocks: uint64(totalBlocks)})
	if err != nil {
		t.Fatalf("Formatting failed: %s", err.Error())
	}

	imageContents := make([]byte, totalBytes)
	bytesRead, err := tmpFile.ReadAt(imageContents, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read image file: %s", err.Error())
	}
	if uint(bytesRead) != totalBytes {
		t.Fatalf("Image size is wrong; expected %d, got %d", totalBytes, bytesRead)
	}

	for i := uint(0); i < totalBytes; i += 128 {
		expectedSector := emptyFloppyImage[i : i+128]
		imageSector := imageContents[i : i+128]
		diffAt := FirstDifference(imageSector, expectedSector)
		if diffAt >= 0 {
			track := (i / 128) / sectorsPerTrack
			trackSector := (i / 128) % sectorsPerTrack
			t.Errorf(
				"Images not equal in absolute sector %d (track: %d, sector: %d);"+
					" first differing byte at index %d; expected %#02x, got %#02x",
				i/128,
				track+1,
				trackSector+1,
				diffAt,
				expectedSector[diffAt],
				imageSector[diffAt],
			)
		}
	}
}

func TestFormattingFloppy(t *testing.T) {
	ValidateImage(t, 2002, 26, emptyFloppyImage)
}

func TestFormattingMiniFloppy(t *testing.T) {
	ValidateImage(t, 640, 16, emptyMinifloppyImage)
}

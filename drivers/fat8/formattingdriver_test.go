package fat8

import (
	_ "embed"
	"io/ioutil"
	"testing"

	"github.com/dargueta/disko"
)

//go:embed test-data/empty-floppy.img
var emptyFloppyImage []byte

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
		t.Errorf(
			"Embedded test data is wrong: image should be %d bytes, got %d",
			totalBytes,
			len(expectedImage),
		)
	}

	tmpFile, err := ioutil.TempFile("", "image.bin")
	if err != nil {
		t.Errorf("Failed to create temporary file: %v", err)
	}

	driver := NewDriverFromFile(tmpFile)
	driver.Format(disko.FSStat{TotalBlocks: uint64(totalBlocks)})

	imageContents := make([]byte, totalBytes)
	bytesRead, err := tmpFile.ReadAt(imageContents[:], 0)
	if err != nil {
		t.Errorf("Failed to read image file: %v", err)
	}
	if uint(bytesRead) != totalBytes {
		t.Errorf("Image size is wrong; expected %d, got %d", totalBytes, bytesRead)
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
				track,
				trackSector,
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

// TODO
func TestFormattingMiniFloppy(t *testing.T) {
	//ValidateImage(t, 720, 13, emptyFloppyImage)
}

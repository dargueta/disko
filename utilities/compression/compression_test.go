package compression_test

import (
	"bytes"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/noxer/bytewriter"
)

func TestRoundTripCompression(t *testing.T) {
	sourceData := make([]byte, 2821)
	//rand.Read(sourceData)
	sourceDataReader := bytes.NewReader(sourceData)

	compressedBuffer := make([]byte, len(sourceData)*2)
	compressedWriter := bytewriter.New(compressedBuffer)

	compressedSize, err := c.CompressImage(sourceDataReader, compressedWriter)
	if err != nil {
		t.Fatalf("unexpected error while compressing: %s", err.Error())
	}
	t.Logf("image size after compression: %d -> %d", len(sourceData), compressedSize)

	decompressedBuffer := make([]byte, len(sourceData))
	decompressedWriter := bytewriter.New(decompressedBuffer)
	compressedReader := bytes.NewReader(compressedBuffer[:compressedSize])

	n, err := c.DecompressImage(compressedReader, decompressedWriter)
	if err != nil {
		t.Fatalf("unexpected error while decompressing: %s", err.Error())
	}
	if n != int64(len(sourceData)) {
		t.Errorf(
			"decompressed image has wrong size: expected %d, got %d",
			len(sourceData),
			n,
		)
	}
	if !bytes.Equal(sourceData, decompressedBuffer) {
		t.Error("original and decompressed data don't match.")
	}
}

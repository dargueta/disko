package compression_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/noxer/bytewriter"
)

type imageC9nTestRunner struct {
	Name     string
	Function func(t *testing.T, d []byte)
}

type imageC9nTestData struct {
	Name string
	Data []byte
}

func TestRoundTripImageCompression(t *testing.T) {
	testRunners := []imageC9nTestRunner{
		{"to_stream", runRoundTripCompressionTest},
		{"to_bytes", runRoundTripCompressionToBytesTest},
	}

	randomData := make([]byte, 119)
	rand.Read(randomData)

	testData := []imageC9nTestData{
		{"homogenous", bytes.Repeat([]byte{100}, 9174)},
		{"empty", []byte{}},
		{"heterogenous", randomData},
	}

	for _, runner := range testRunners {
		t.Run(
			runner.Name,
			func(tSub *testing.T) {
				for _, data := range testData {
					tSub.Run(
						data.Name,
						func(tSubSub *testing.T) {
							runner.Function(tSubSub, data.Data)
						},
					)
				}
			},
		)
	}
}

func runRoundTripCompressionTest(t *testing.T, sourceData []byte) {
	sourceDataReader := bytes.NewReader(sourceData)

	compressedBuffer := make([]byte, 10240)
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

func runRoundTripCompressionToBytesTest(t *testing.T, originalData []byte) {
	compressed, err := c.CompressImageToBytes(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("unexpected error while compressing: %s", err.Error())
	}
	t.Logf("image compressed %d -> %d", len(originalData), len(compressed))

	decompressed, err := c.DecompressImageToBytes(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("unexpected error while decompressing: %s", err.Error())
	}

	if len(originalData) != len(decompressed) {
		t.Errorf(
			"decompressed data length doesn't match; expected %d, got %d",
			len(originalData),
			len(decompressed),
		)
	}
	if !bytes.Equal(originalData, decompressed) {
		t.Errorf("decompressed data doesn't match original")
	}
}

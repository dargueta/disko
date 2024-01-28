package compression_test

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/noxer/bytewriter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type imageC9nTestRunner struct {
	Name     string
	Function func(t *testing.T, d []byte)
}

type imageC9nTestData struct {
	Name string
	Data []byte
}

// compressImageToBytes is a convenience function wrapping [CompressImage]. It
// functions identically, except it returns the compressed data in a new byte
// slice instead of writing to an [io.Writer].
func compressImageToBytes(input io.Reader) ([]byte, error) {
	buffer := bytes.Buffer{}
	writer := bufio.NewWriter(&buffer)
	_, err := c.CompressImage(input, writer)
	if err != nil {
		return nil, err
	}

	writer.Flush()

	outputSlice := make([]byte, buffer.Len())
	copy(outputSlice, buffer.Bytes())
	return outputSlice, nil
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
	require.NoError(t, err, "unexpected error while compressing")
	t.Logf("image size after compression: %d -> %d", len(sourceData), compressedSize)

	decompressedBuffer := make([]byte, len(sourceData))
	decompressedWriter := bytewriter.New(decompressedBuffer)
	compressedReader := bytes.NewReader(compressedBuffer[:compressedSize])

	n, err := c.DecompressImage(compressedReader, decompressedWriter)
	require.NoError(t, err, "unexpected error while decompressing")
	assert.EqualValues(t, len(sourceData), n, "decompressed image has wrong size")
	assert.Equal(t, sourceData, decompressedBuffer, "decompressed data is wrong")
}

func runRoundTripCompressionToBytesTest(t *testing.T, originalData []byte) {
	compressed, err := compressImageToBytes(bytes.NewReader(originalData))
	require.NoError(t, err, "error while compressing")
	t.Logf("image compressed %d -> %d", len(originalData), len(compressed))

	decompressed, err := c.DecompressImageToBytes(bytes.NewReader(compressed))
	require.NoError(t, err, "error while decompressing")

	assert.Equal(
		t, len(originalData), len(decompressed), "decompressed data length is wrong")
	assert.Equal(t, originalData, decompressed, "decompressed data is wrong")
}

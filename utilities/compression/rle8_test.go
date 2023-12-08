package compression_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/noxer/bytewriter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RLE8TestCase struct {
	Input          []byte
	ExpectedOutput []byte
	Name           string
}

func TestCompressRLE8__Basic(t *testing.T) {
	tests := []RLE8TestCase{
		{[]byte{}, []byte{}, "empty"},
		{[]byte{4, 4}, []byte{4, 4, 0}, "run with two only"},
		{[]byte{0, 1, 2, 3, 4}, []byte{0, 1, 2, 3, 4}, "no runs"},
		{[]byte{6, 1, 3, 0, 0}, []byte{6, 1, 3, 0, 0, 0}, "two at end"},
		{[]byte{6, 1, 0, 0, 0}, []byte{6, 1, 0, 0, 1}, "three at end"},
		{[]byte{9, 5, 5, 5, 5, 5, 3, 7}, []byte{9, 5, 5, 3, 3, 7}, "short run"},
		{
			[]byte{9, 5, 5, 5, 5, 5, 5, 3, 3, 3, 3, 7, 2, 6},
			[]byte{9, 5, 5, 4, 3, 3, 2, 7, 2, 6},
			"adjacent runs",
		},
		{
			bytes.Repeat([]byte{5}, 1024),
			[]byte{5, 5, 255, 5, 5, 255, 5, 5, 255, 5, 5, 251},
			"single long run",
		},
		{
			bytes.Repeat([]byte{8}, 257),
			[]byte{8, 8, 255},
			"257",
		},
		{
			bytes.Repeat([]byte{8}, 258),
			[]byte{8, 8, 255, 8},
			"258",
		},
		{
			bytes.Repeat([]byte{8}, 259),
			[]byte{8, 8, 255, 8, 8, 0},
			"259",
		},
	}

	for _, test := range tests {
		t.Run(
			test.Name,
			func(t *testing.T) {
				runCompressionTestCase(t, test)
			},
		)
	}
}

// Round-trip test of completely random bytes
func TestRLE8RoundTrip__CompletelyRandom(t *testing.T) {
	originalData := make([]byte, 1852)
	rand.Read(originalData)
	runRoundTripTestCase(t, originalData)
}

func TestRLE8RoundTrip__EntirelyNulls(t *testing.T) {
	originalData := make([]byte, 571)
	runRoundTripTestCase(t, originalData)
}

func TestRLE8RoundTrip__EntirelyNonNullRun(t *testing.T) {
	runRoundTripTestCase(t, bytes.Repeat([]byte{182}, 934))
}

func TestRLE8RoundTrip__Empty(t *testing.T) {
	runRoundTripTestCase(t, []byte{})
}

func TestRLE8Decompress__MissingRepeatCount(t *testing.T) {
	data := []byte{9, 1, 4, 4}
	decompressed := make([]byte, 16)
	writer := bytewriter.New(decompressed)

	_, err := c.DecompressRLE8(bytes.NewReader(data), writer)
	assert.Error(t, err, "read with missing repeat count should've failed but didn't")
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "error should wrap io.ErrUnexpectedEOF")
}

////////////////////////////////////////////////////////////////////////////////
// Helper functions

func runCompressionTestCase(t *testing.T, test RLE8TestCase) {
	inputBuffer := bytes.NewBuffer(test.Input)
	outputBuffer := make([]byte, len(test.ExpectedOutput)*2)
	outputWriter := bytewriter.New(outputBuffer)

	n, err := c.CompressRLE8(inputBuffer, outputWriter)

	assert.NoError(t, err)
	assert.EqualValues(t, len(test.ExpectedOutput), n, "# bytes written is wrong")
	assert.Truef(
		t,
		bytes.Equal(test.ExpectedOutput, outputBuffer[:n]),
		"output data is wrong: expected %q, got %q",
		test.ExpectedOutput,
		outputBuffer)
}

func runRoundTripTestCase(t *testing.T, originalData []byte) {
	inputBuffer := bytes.NewBuffer(originalData)

	// If the source data is sufficiently random, the "compressed" data can
	// actually be larger than the input. Thus, we need to make the compressed
	// buffer larger than the input.
	compressedBuffer := make([]byte, len(originalData)*2)
	compressedWriter := bytewriter.New(compressedBuffer)

	n, err := c.CompressRLE8(inputBuffer, compressedWriter)
	require.NoError(t, err, "error while compressing")

	t.Logf("compressed %d to %d", len(originalData), n)
	outputBuffer := make([]byte, len(originalData))
	outputWriter := bytewriter.New(outputBuffer)
	compressedReader := bytes.NewReader(compressedBuffer[:n])

	n, err = c.DecompressRLE8(compressedReader, outputWriter)
	assert.NoError(t, err, "error while decompressing")
	assert.EqualValues(t, len(originalData), n, "returned decompressed size is wrong")
	assert.True(
		t,
		bytes.Equal(originalData, outputBuffer),
		"decompressed data doesn't match original")
}

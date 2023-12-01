package testing

import (
	"bytes"
	"io"
	"testing"

	"github.com/dargueta/disko/utilities/compression"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/bytesextra"
)

// LoadDiskImage takes a compressed disk image and returns a stream to access the
// uncompressed data.
//
//   - Writes to the stream do not affect `compressedImageBytes`.
//   - While the stream can be written to, its size is fixed to `sectorSize * totalSectors`.
//     Attempting to write past the end of this buffer will trigger an error.
func LoadDiskImage(
	t *testing.T, compressedImageBytes []byte, sectorSize, totalSectors uint,
) io.ReadWriteSeeker {
	compressedBuf := bytes.NewBuffer(compressedImageBytes)
	require.Greater(t, len(compressedImageBytes), 0, "compressed image is empty")

	imageBytes, err := compression.DecompressImageToBytes(compressedBuf)
	require.NoError(t, err)

	require.Equal(
		t,
		totalSectors*sectorSize,
		uint(len(imageBytes)),
		"uncompressed image is wrong size",
	)
	return bytesextra.NewReadWriteSeeker(imageBytes)
}

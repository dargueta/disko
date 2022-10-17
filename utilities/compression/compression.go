package compression

import (
	"compress/gzip"
	"io"
)

// CompressImage compresses a disk image using RLE8 and gzip.
//
// The returned int64 gives the number of bytes written to the output stream. If
// an error occurred, the value is undefined and should not be used.
func CompressImage(input io.Reader, output io.Writer) (int64, error) {
	// Wrap the output stream in a gzip compressor using the highest compression
	// available. The disk images aren't that huge so we won't notice much of a
	// speed difference between the default and highest levels.
	gzWriter, err := gzip.NewWriterLevel(output, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	defer gzWriter.Close()

	return CompressRLE8(input, gzWriter)
}

// DecompressImage takes a gzipped, RLE8-encoded disk image and decompresses it
// to the original raw bytes.
//
// The returned int64 gives the number of bytes written to the output (i.e. the
// decompressed size of the image). If an error occurred, the value is undefined
// and should not be used.
func DecompressImage(input io.Reader, output io.Writer) (int64, error) {
	gzReader, err := gzip.NewReader(input)
	if err != nil {
		return 0, err
	}
	defer gzReader.Close()
	return DecompressRLE8(gzReader, output)
}

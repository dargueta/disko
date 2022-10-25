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
	// Because we have no way of getting the number of bytes written to the
	// output stream from an io.Writer, we need to keep track of it ourselves.
	bytesWritten := int64(0)
	writer := countingWriter{Writer: output, PBytesWritten: &bytesWritten}

	// Wrap the output stream in a gzip compressor using the highest compression
	// available. The disk images aren't that huge by modern standards (mostly
	// under 32MiB), so we won't notice much of a speed difference between the
	// default and highest levels.
	gzWriter, err := gzip.NewWriterLevel(writer, gzip.BestCompression)
	if err != nil {
		return 0, err
	}

	_, err = CompressRLE8(input, gzWriter)
	closeErr := gzWriter.Close()
	if err != nil {
		return bytesWritten, err
	}
	if closeErr != nil {
		return bytesWritten, closeErr
	}
	return bytesWritten, nil
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

// countingWriter is a wrapper around [io.Writer] streams that keeps track of
// how many bytes are successfully written to the stream. Because a Writer doesn't
// take a pointer receiver, you need to pass this a pointer to an integer you
// maintain to get the count.
type countingWriter struct {
	// Writer is the [io.Writer] that this intercepts the writes to.
	Writer io.Writer

	// PBytesWritten is a pointer to an integer that will be updated on every
	// successful write with the total number of bytes written to Writer.
	PBytesWritten *int64
}

// Write writes bytes to the underlying stream.
func (w countingWriter) Write(b []byte) (int, error) {
	n, err := w.Writer.Write(b)
	if err == nil {
		*w.PBytesWritten += int64(n)
	}
	return n, err
}

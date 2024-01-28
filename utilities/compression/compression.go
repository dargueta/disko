package compression

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// CompressImage compresses a disk image using RLE8 and gzip.
//
// The returned int64 gives the number of bytes written to the output stream. If
// an error occurred, this value is undefined and should not be used.
func CompressImage(input io.Reader, output io.Writer) (int64, error) {
	// Because we have no way of getting the number of bytes written to the
	// output stream from an io.Writer, we need to keep track of it ourselves.
	writer := countingWriter{Writer: output}

	// Wrap the output stream in a gzip compressor using the highest compression
	// available. The disk images aren't that huge by modern standards (mostly
	// under 32MiB), so we won't notice much of a speed difference between the
	// default and highest levels.
	gzWriter, err := gzip.NewWriterLevel(&writer, gzip.BestCompression)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	_, err = CompressRLE8(input, gzWriter)
	closeErr := gzWriter.Close()
	if err != nil {
		err = fmt.Errorf("RLE8 compression error: %w", err)
	} else if closeErr != nil {
		err = fmt.Errorf("gzip compression error: %w", closeErr)
	}
	return writer.BytesWritten, err
}

// DecompressImage takes a gzipped, RLE8-encoded byte stream and decompresses it
// to the original data.
//
// The returned int64 gives the number of bytes written to the output (i.e. the
// decompressed size of the data). If an error occurred, the value is undefined
// and should not be used.
func DecompressImage(input io.Reader, output io.Writer) (int64, error) {
	gzReader, err := gzip.NewReader(input)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()
	return DecompressRLE8(gzReader, output)
}

// DecompressImageToBytes is a convenience function wrapping [DecompressImage].
// It functions identically, except it returns the decompressed data in a new
// byte slice instead of writing to an [io.Writer]. It's most useful for reading
// embedded test data.
func DecompressImageToBytes(input io.Reader) ([]byte, error) {
	buffer := bytes.Buffer{}
	writer := bufio.NewWriter(&buffer)
	_, err := DecompressImage(input, writer)
	if err != nil {
		return nil, err
	}

	writer.Flush()

	outputSlice := make([]byte, buffer.Len())
	copy(outputSlice, buffer.Bytes())
	return outputSlice, nil
}

// countingWriter is a wrapper around [io.Writer] streams that keeps track of
// how many bytes are successfully written to the stream.
type countingWriter struct {
	// Writer is the [io.Writer] that this intercepts the writes to.
	Writer io.Writer

	// BytesWritten is the total number of bytes successfully written to [Writer].
	BytesWritten int64
}

// Write writes bytes to the underlying stream.
func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.Writer.Write(b)
	if err == nil {
		w.BytesWritten += int64(n)
	}
	return n, err
}

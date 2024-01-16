package compression

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// CompressRLE8 reads bytes from the input and writes compressed data from the
// output until the input is exhausted. The return value is the number of bytes
// written, only valid if no error occurred.
func CompressRLE8(input io.Reader, output io.Writer) (int64, error) {
	grouper := NewRLEGrouper(input)

	totalBytesWritten := int64(0)
	for {
		run, getRunErr := grouper.GetNextRun()
		if getRunErr != nil && !errors.Is(getRunErr, io.EOF) {
			// An error was encountered and it's *not* EOF.
			return totalBytesWritten, getRunErr
		}

		for run.RunLength >= 2 {
			var repeatCount int
			if run.RunLength > 257 {
				repeatCount = 255
			} else {
				repeatCount = run.RunLength - 2
			}

			n, err := output.Write([]byte{run.Byte, run.Byte, byte(repeatCount)})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += int64(n)
			run.RunLength -= repeatCount + 2
		}

		if run.RunLength == 1 {
			n, err := output.Write([]byte{run.Byte})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += int64(n)
		}

		// We bail at the beginning of the loop if an error occurred and it's
		// *not* EOF, so if the error here is non-nil then that means it *must*
		// be EOF. That means we finished without errors.
		if getRunErr != nil {
			return totalBytesWritten, nil
		}
	}
}

func DecompressRLE8(input io.Reader, output io.Writer) (int64, error) {
	source := bufio.NewReader(input)
	lastByteRead := -1
	totalBytesWritten := int64(0)

	for {
		currentByte, err := source.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return totalBytesWritten, nil
			}
			return totalBytesWritten, fmt.Errorf("error reading input: %w", err)
		}

		var currentOutput []byte
		if int(currentByte) == lastByteRead {
			// Got two bytes in a row that are the same. The next byte is a repeat
			// count.
			repeatCountByte, err := source.ReadByte()
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = fmt.Errorf(
						"%w: missing repeat count after two %02x bytes",
						io.ErrUnexpectedEOF,
						uint(lastByteRead),
					)
				}
				return totalBytesWritten, fmt.Errorf("failed to write to output: %w", err)
			}

			// Note we're writing out repeatCount + 1 instead of +2. We do this
			// because on the previous iteration of the loop we already wrote it
			// out once.
			currentOutput = bytes.Repeat([]byte{currentByte}, int(repeatCountByte)+1)

			// Reset the last byte read since we're done with this group. If we
			// didn't do this, runs of 258+ bytes would be decompressed
			// incorrectly, adding in extra bytes.
			lastByteRead = -1
		} else {
			lastByteRead = int(currentByte)
			currentOutput = []byte{currentByte}
		}

		n, err := output.Write(currentOutput)
		if err != nil {
			return totalBytesWritten, fmt.Errorf("failed to write to output: %w", err)
		}
		totalBytesWritten += int64(n)
	}
}

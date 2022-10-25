package compression

import (
	"bufio"
	"bytes"
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
		run, err := grouper.GetNextRun()
		if err == io.EOF {
			return totalBytesWritten, nil
		} else if err != nil {
			return totalBytesWritten, err
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
			output.Write([]byte{run.Byte})
			totalBytesWritten++
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
			if err == io.EOF {
				return totalBytesWritten, nil
			}
			return totalBytesWritten, err
		}

		var currentOutput []byte
		if int(currentByte) == lastByteRead {
			// Got two bytes in a row that are the same. The next byte is a repeat
			// count.
			repeatCountByte, err := source.ReadByte()
			if err != nil {
				if err == io.EOF {
					return totalBytesWritten,
						fmt.Errorf("hit unexpected EOF: byte run missing repeat count")
				}
				return totalBytesWritten, err
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
			return totalBytesWritten, err
		}
		totalBytesWritten += int64(n)
	}
}

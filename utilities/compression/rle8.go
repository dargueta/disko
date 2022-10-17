package compression

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// CompressRLE8
func CompressRLE8(input io.Reader, output io.Writer) (int64, error) {
	grouper := NewRunLengthGrouper(input)

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
				run.RunLength -= 255
			} else {
				repeatCount = run.RunLength - 2
				run.RunLength = 0
			}

			n, err := output.Write([]byte{run.Byte, run.Byte, byte(repeatCount)})
			totalBytesWritten += int64(n)
			run.RunLength -= repeatCount + 2
			if err != nil {
				return totalBytesWritten, err
			}
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

			currentOutput = bytes.Repeat([]byte{currentByte}, int(repeatCountByte)+1)
		} else {
			lastByteRead = int(currentByte)
			currentOutput = []byte{currentByte}
		}

		n, err := output.Write(currentOutput)
		totalBytesWritten += int64(n)
		if err != nil {
			return totalBytesWritten, err
		}
	}
}

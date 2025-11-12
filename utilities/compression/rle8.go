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
	grouper := NewRLEGrouperFromReader(input)

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

// CompressRLERecursive is similar to CompressRLE8, but uses recursive run-length
// encoding.
//
// It's identical to normal RLE8 for runs <= 256 bytes, but gets better compression
// for runs over 258 bytes. It also has the advantage that long runs can themselves
// be run-length encoded. For example, consider a run of 2048 nulls. This would
// take 24 bytes with normal RLE8, or 11 bytes with recursive RLE:
//
//   - RLE8 : `00 00 FF 00 00 FF 00 00 FF 00 00 FF 00 00 FF 00 00 FF 00 00 FF 00 00 F7`
//   - RRLE : `00 00 FF FF FF FF FF FF FF FF 06`
//   - RRLE²: `00 00 00 FF FF 06 06`
//
// The contrast is even more stark for huge runs (say 128 KiB as part of a 7" floppy).
// Using RRLE it would be 517 bytes: `00 00 FF{514 times} 00`, giving a compression
// ratio of about 253:1. Because of the long run of FF, we can run RRLE on this again
// and get to 9 bytes, a ratio of about 14,564:1. By contrast, RLE8 gives a compressed
// size of 1530 bytes (86:1) and can't be reduced further.
func CompressRLERecursive(input io.Reader, output io.Writer) (int64, error) {
	grouper := NewRLEGrouperFromReader(input)

	totalBytesWritten := int64(0)
	for {
		run, getRunErr := grouper.GetNextRun()
		if getRunErr != nil && !errors.Is(getRunErr, io.EOF) {
			// An error was encountered and it's *not* EOF.
			return totalBytesWritten, getRunErr
		}

		if run.RunLength == 1 {
			_, err := output.Write([]byte{run.Byte})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten++
		} else if run.RunLength >= 2 {
			_, err := output.Write([]byte{run.Byte, run.Byte})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += 2

			num255Repetitions := (run.RunLength - 2) / 255
			remainder := (run.RunLength - 2) % 255

			_, err = output.Write(bytes.Repeat([]byte{255}, num255Repetitions))
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += int64(num255Repetitions)

			_, err = output.Write([]byte{byte(remainder)})
			if err != nil {
				return totalBytesWritten, err
			}

			totalBytesWritten++
		}

		// We bail at the beginning of the loop if an error occurred and it's
		// *not* EOF, so if the error here is non-nil then that means it *must*
		// be EOF. That means we finished without errors.
		if getRunErr != nil {
			return totalBytesWritten, nil
		}
	}
}

// DecompressRLERecursive is the decompression counterpart to CompressRLERecursive.
func DecompressRLERecursive(input io.Reader, output io.Writer) (int64, error) {
	grouper := NewRLEGrouperFromReader(input)

	totalBytesWritten := int64(0)
	for {
		run, getRunErr := grouper.GetNextRun()
		if getRunErr != nil && !errors.Is(getRunErr, io.EOF) {
			// An error was encountered and it's *not* EOF.
			return totalBytesWritten, getRunErr
		}

		if run.RunLength == 1 {
			_, err := output.Write([]byte{run.Byte})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten++
		} else if run.RunLength >= 2 {
			_, err := output.Write([]byte{run.Byte, run.Byte})
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += 2

			num255Repetitions := (run.RunLength - 2) / 255
			remainder := (run.RunLength - 2) % 255

			_, err = output.Write(bytes.Repeat([]byte{255}, num255Repetitions))
			if err != nil {
				return totalBytesWritten, err
			}
			totalBytesWritten += int64(num255Repetitions)

			_, err = output.Write([]byte{byte(remainder)})
			if err != nil {
				return totalBytesWritten, err
			}

			totalBytesWritten++
		}

		// We bail at the beginning of the loop if an error occurred and it's
		// *not* EOF, so if the error here is non-nil then that means it *must*
		// be EOF. That means we finished without errors.
		if getRunErr != nil {
			return totalBytesWritten, nil
		}
	}
}

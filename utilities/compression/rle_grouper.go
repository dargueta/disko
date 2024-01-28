package compression

import (
	"bufio"
	"errors"
	"io"
	"math"
)

// ByteRun represents a single run of a particular byte value.
type ByteRun struct {
	// Byte is the byte value for this run.
	Byte byte
	// RunLength gives the number of times the byte occurs in the run.
	//
	// A valid run will always have this be 1 or greater. A value less than 1
	// indicates either EOF was encountered, or an error occurred.
	RunLength int
}

// InvalidRLERun is a sentinel value returned by [RLEGrouper.GetNextRun] if an
// error occurred, or EOF was encountered.
var InvalidRLERun = ByteRun{0, 0}

// An RLEGrouper wraps an [io.Reader] and returns a [ByteRun] upon reads.
//
// This functions much like the `uniq` command line utility.
type RLEGrouper struct {
	rd io.ByteScanner
}

// NewRLEGrouperFromReader constructs an [RLEGrouper] from an [io.Reader].
func NewRLEGrouperFromReader(rd io.Reader) RLEGrouper {
	return NewRLEGrouperFromByteScanner(bufio.NewReader(rd))
}

// NewRLEGrouper constructs an [RLEGrouper] from an [io.ByteScanner].
func NewRLEGrouperFromByteScanner(rd io.ByteScanner) RLEGrouper {
	return RLEGrouper{rd: rd}
}

// GetNextRun returns a [ByteRun] for the next byte or run of byte values in the
// stream. The length of a valid run is guaranteed to be in the range [1, math.MaxInt).
// A valid run will never have length 0.
//
// The returned error behaves identically to [io.Reader.Read], namely that if
// the returned run length is non-zero, the error will either be nil or [io.EOF].
// If it's zero, the error is either [io.EOF] or another (non-nil) error.
func (grouper RLEGrouper) GetNextRun() (ByteRun, error) {
	firstByte, err := grouper.rd.ReadByte()
	// Bail if any error occurred, including EOF.
	if err != nil {
		return InvalidRLERun, err
	}

	runLength := 1
	for ; runLength < math.MaxInt; runLength++ {
		currentByte, err := grouper.rd.ReadByte()

		// If we get EOF as the error from ReadByte() then that means that we
		// reached the end of the file on the previous read. On this read,
		// currentByte is invalid.
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Hit EOF. If we get here then the previous byte we read was part
				// of the current run, so we don't unread the last byte we saw.
				return ByteRun{Byte: firstByte, RunLength: runLength}, io.EOF
			}
			// Some other error we weren't expecting occurred.
			return InvalidRLERun, err
		}

		if currentByte != firstByte {
			// Hit a different byte, back up and return.
			grouper.rd.UnreadByte()
			return ByteRun{Byte: firstByte, RunLength: runLength}, nil
		}
	}

	// In the extremely unlikely event we hit the maximum size for a signed int
	// before the end of the run, we return early to avoid overflow.
	return ByteRun{Byte: firstByte, RunLength: runLength}, nil
}

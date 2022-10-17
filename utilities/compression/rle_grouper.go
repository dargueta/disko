package compression

import (
	"bufio"
	"io"
)

// ByteRun represents a single run of a particular byte value.
type ByteRun struct {
	// Byte is the byte value for this run.
	Byte byte
	// RunLength gives the number of times the byte occurs in the run (not the
	// number of times it's repeated).
	//
	// A valid run will always have this be 1 or greater. A value less than 1
	// indicates either EOF was encountered, or an error occurred.
	RunLength int
}

// RunLengthGrouper returns a
type RunLengthGrouper struct {
	rd *bufio.Reader
}

func NewRunLengthGrouper(rd io.Reader) RunLengthGrouper {
	return RunLengthGrouper{rd: bufio.NewReader(rd)}
}

// GetNextRun returns a [ByteRun] for the next byte or run of byte values in the
// stream.
func (grouper RunLengthGrouper) GetNextRun() (ByteRun, error) {
	firstByte, err := grouper.rd.ReadByte()
	// Bail if any error occurred, including EOF.
	if err != nil {
		return ByteRun{Byte: 0, RunLength: 0}, err
	}

	var runLength int
	for runLength = 1; ; runLength++ {
		currentByte, err := grouper.rd.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return ByteRun{Byte: 0, RunLength: 0}, err
		}
		if currentByte != firstByte {
			// Hit a different byte, back up and return.
			grouper.rd.UnreadByte()
			break
		}
	}
	return ByteRun{Byte: firstByte, RunLength: runLength}, nil
}

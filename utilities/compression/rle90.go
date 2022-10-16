package utilities

import (
	"bytes"
	"io"
)

type RLE90Reader struct {
	io.ReadCloser
	stream               io.ByteReader
	lastByte             byte
	remainingRepeatCount int
}

type RLE90Writer struct {
	io.WriteCloser
	stream            io.Writer
	lastByte          int
	lastByteRunLength int
}

// NewReader returns an io.Reader which decompresses RLE90-encoded data from rd.
func NewRLE90Reader(rd io.ByteReader) (RLE90Reader, error) {
	return RLE90Reader{stream: rd}, nil
}

// BUG(dargueta): This should fail if a stream starts with 90H followed by a non-zero byte.
func (reader *RLE90Reader) Read(p []byte) (int, error) {
	var writeSize int
	var sliceToWrite []byte
	numBytesRead := 0

	// Copy data we've expanded but didn't read into the output buffer
	if reader.remainingRepeatCount > 0 {
		if len(p) > reader.remainingRepeatCount {
			writeSize = reader.remainingRepeatCount
		} else {
			writeSize = len(p)
		}

		sliceToWrite = bytes.Repeat([]byte{reader.lastByte}, writeSize)
		copy(p, sliceToWrite)
		numBytesRead += writeSize
		reader.remainingRepeatCount -= writeSize
	}

	for numBytesRead < len(p) {
		nextByte, err := reader.stream.ReadByte()
		if err == io.EOF {
			// If we hit EOF at the beginning of the loop then this isn't an error.
			// It's only an error if we hit EOF immediately after a 0x90 byte.
			return numBytesRead, io.EOF
		} else if err != nil {
			// Didn't hit EOF, must've been an I/O error.
			return numBytesRead, err
		}

		if nextByte != '\x90' {
			reader.lastByte = nextByte
			p[numBytesRead] = nextByte
			numBytesRead++
			continue
		}

		// Hit a sentinel, expecting another byte indicating the repeat count.
		repeatCountByte, err := reader.stream.ReadByte()
		if err != nil {
			// Hit EOF after a repeat count -- this is an error.
			return 0, io.ErrUnexpectedEOF
		}

		repeatCount := int(repeatCountByte)
		if repeatCount == 0 {
			// Escape sequence 0x90 0x00 gives 0x90
			p[numBytesRead] = '\x90'
			reader.lastByte = '\x90'
			numBytesRead++
		} else {
			remainingSpace := len(p) - numBytesRead
			if remainingSpace < repeatCount {
				// Buffer doesn't have enough space for the remaining duplicated
				// bytes.
				writeSize = remainingSpace
				reader.remainingRepeatCount = repeatCount - remainingSpace
			} else {
				// Buffer has enough space for the repeated byte count.
				writeSize = repeatCount
				reader.remainingRepeatCount = 0
			}

			sliceToWrite := bytes.Repeat([]byte{reader.lastByte}, writeSize)
			copy(p[numBytesRead:numBytesRead+writeSize], sliceToWrite)
			numBytesRead += writeSize
		}
	}

	if numBytesRead < len(p) {
		return numBytesRead, io.EOF
	}
	return numBytesRead, nil
}

// ReadAll unpacks the remainder of the data in the reader and returns it as a
// byte array.
func (reader *RLE90Reader) ReadAll() ([]byte, error) {
	fullContents := new(bytes.Buffer)
	var intermediateBuffer [512]byte

	for {
		sizeWritten, err := reader.Read(intermediateBuffer[:])
		if err != nil {
			return fullContents.Bytes(), err
		}

		fullContents.Write(intermediateBuffer[:sizeWritten])
		if sizeWritten < len(intermediateBuffer) {
			return fullContents.Bytes(), nil
		}
	}
}

func (reader *RLE90Reader) Close() error {
	return nil
}

func NewWriter(stream io.Writer) (RLE90Writer, error) {
	return RLE90Writer{stream: stream, lastByte: -1}, nil
}

// Write writes the byte slice to the underlying reader.
func (writer *RLE90Writer) Write(p []byte) (int, error) {
	numWritten := 0

	for _, nextByte := range p {
		if int(nextByte) == writer.lastByte {
			// Current byte is same as the last byte. Increment the run length
			// but don't actually write anything to the output, since this may
			// not be the end of the run.
			writer.lastByteRunLength++
		} else if writer.lastByteRunLength >= 1 {
			// Current byte is different from the last byte and we have at least two
			// consecutive bytes with the same value.
			writer.Flush()
			writer.lastByte = int(nextByte)
		} else {
			// Current byte is different from the last byte and run length = 0.
			writer.lastByte = int(nextByte)
			writer.stream.Write([]byte{nextByte})
		}
		numWritten++
	}
	return numWritten, nil
}

func (writer *RLE90Writer) writeDuplicatedByte(value byte, count int) error {
	for count > 3 {
		var nextWriteCount int
		if count > 254 {
			nextWriteCount = 254
		} else {
			nextWriteCount = count
		}

		_, err := writer.stream.Write([]byte{0x90, byte(nextWriteCount + 1)})
		if err != nil {
			return err
		}

		count -= nextWriteCount
	}

	if count > 0 {
		sliceToWrite := bytes.Repeat([]byte{value}, count)
		_, err := writer.stream.Write(sliceToWrite)
		return err
	}
	return nil
}

func (writer *RLE90Writer) Flush() error {
	err := writer.writeDuplicatedByte(byte(writer.lastByte), writer.lastByteRunLength+1)
	if err != nil {
		return err
	}
	writer.lastByte = -1
	writer.lastByteRunLength = 0
	return nil
}

type Flusher interface {
	Flush() error
}

func (writer *RLE90Writer) Close() error {
	// TODO (dargueta): How do we flush the underlying stream?
	return writer.Flush()
}

func CompressBytes(unpacked []byte) ([]byte, error) {
	var targetBuffer bytes.Buffer
	_ = io.Writer(&targetBuffer)

	writer, err := NewWriter(&targetBuffer)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(unpacked)
	return targetBuffer.Bytes(), err
}

func DecompressBytes(packed []byte) ([]byte, error) {
	var packedCopy []byte
	copy(packedCopy, packed)
	stream := bytes.NewReader(packedCopy)
	reader, err := NewRLE90Reader(stream)
	if err != nil {
		return nil, err
	}
	return reader.ReadAll()
}

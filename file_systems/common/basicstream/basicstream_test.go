package basicstream_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/file_systems/common/basicstream"
	diskotest "github.com/dargueta/disko/testing"
)

// SeekInfo is a struct useful for testing seeking in  a stream using relative
// offsets.
type SeekInfo struct {
	Offset                int64
	Whence                int
	ExpectedFinalPosition int64
}

// Read the entire image all at once
func TestBasicStreamNew__Basic(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 256, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	if err != nil {
		t.Fatalf("couldn't create stream: %s", err.Error())
	}

	rawExpectedData, err := cache.Data()
	if err != nil {
		t.Fatalf("failed to get cache data as slice: %s", err.Error())
	}

	streamData := make([]byte, cache.Size())
	n, err := stream.Read(streamData)
	if n != int(cache.Size()) {
		t.Errorf(
			"read wrong number of bytes from stream: expected %d bytes, got %d",
			cache.Size(),
			n,
		)
	}
	if err != nil {
		t.Fatalf("failed to read entire stream contents: %s", err.Error())
	}
	if !bytes.Equal(rawExpectedData, streamData) {
		t.Error("data read from stream is not equal to the expected raw data")
	}
}

// Read less than one block (ensures rounding is correct)
func TestBasicStreamNew__LessThanOneBlock(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 16, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	if err != nil {
		t.Fatalf("couldn't create stream: %s", err.Error())
	}

	rawExpectedData, err := cache.GetSlice(0, 1)
	if err != nil {
		t.Fatalf("failed to get cache data as slice: %s", err.Error())
	}

	const readSize = 39

	streamData := make([]byte, readSize)
	n, err := stream.Read(streamData)
	if n != int(39) {
		t.Errorf(
			"read wrong number of bytes from stream: expected %d bytes, got %d",
			readSize,
			n,
		)
	}
	if err != nil {
		t.Fatalf("failed to read %d bytes from stream: %s", readSize, err.Error())
	}
	if !bytes.Equal(rawExpectedData[:readSize], streamData) {
		t.Error("data read from stream is not equal to the expected raw data")
	}
}

// Read various sizes at various offsets from the beginning of the stream. This
// only tests Seek() with [io.SeekStart] as `whence`.
func TestBasicStream__SeekStart(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 16, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	if err != nil {
		t.Fatalf("failed to create stream: %s", err.Error())
	}

	byteOffsets := []int64{
		// Beginning of the stream
		0,
		// Exactly at the second block
		128,
		// Within the first block
		90,
		// Starting within the third block
		300,
	}
	readSizes := []int{
		// One byte
		1,
		// Part of a block
		93,
		// One full block
		128,
		// Multiple blocks
		829,
	}

	rawUnderlyingBytes, _ := cache.Data()
	for _, readSize := range readSizes {
		for _, offset := range byteOffsets {
			info := SeekInfo{
				Offset:                offset,
				Whence:                io.SeekStart,
				ExpectedFinalPosition: offset,
			}

			// Run this independently, as a sub-test.
			testName := fmt.Sprintf("Offset_%d_Size_%d", offset, readSize)
			t.Run(
				testName,
				func(subT *testing.T) {
					checkStreamRead(stream, info, readSize, rawUnderlyingBytes, subT)
				},
			)
		}
	}
}

// Hopping around
func TestBasicStream__SeekJumpingAround(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 8, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	if err != nil {
		t.Fatalf("failed to create stream: %s", err.Error())
	}

	seeks := []SeekInfo{
		{
			Offset:                10,
			Whence:                io.SeekStart,
			ExpectedFinalPosition: 10,
		},
		// Seek backwards from the current position
		{
			Offset:                -3,
			Whence:                io.SeekCurrent,
			ExpectedFinalPosition: 7,
		},
		// Don't go anywhere
		{
			Offset:                0,
			Whence:                io.SeekCurrent,
			ExpectedFinalPosition: 7,
		},
		// Seek forwards from the current position
		{
			Offset:                30,
			Whence:                io.SeekCurrent,
			ExpectedFinalPosition: 37,
		},
		// Seek from the end
		{
			Offset:                -39,
			Whence:                io.SeekEnd,
			ExpectedFinalPosition: cache.Size() - 39,
		},
		// Allow seeking past the end of the stream from the current position
		{
			Offset:                102,
			Whence:                io.SeekCurrent,
			ExpectedFinalPosition: cache.Size() - 39 + 102,
		},
		// Go backwards, still past the end of the stream
		{
			Offset:                -17,
			Whence:                io.SeekCurrent,
			ExpectedFinalPosition: cache.Size() - 39 + 102 - 17,
		},
		// Go to the beginning
		{
			Offset:                0,
			Whence:                io.SeekStart,
			ExpectedFinalPosition: 0,
		},
	}

	for _, seek := range seeks {
		_, err := doCheckedSeek(stream, seek, t)
		if err != nil {
			t.Error(err)
		}
	}
}

// doCheckedSeek performs the requested seek on the stream and checks the results.
// If the seek fails, it will return an error. The returned offset is always the
// return value of Seek(), even if it failed.
func doCheckedSeek(stream *basicstream.BasicStream, seek SeekInfo, t *testing.T) (int64, error) {
	originalAbsolutePosition := stream.Tell()
	where, err := stream.Seek(seek.Offset, seek.Whence)
	if err != nil {
		return where, fmt.Errorf(
			"failed to seek from %d to %d using offset %d, origin %d: %s",
			originalAbsolutePosition,
			seek.ExpectedFinalPosition,
			seek.Offset,
			seek.Whence,
			err.Error(),
		)
	} else if where != seek.ExpectedFinalPosition {
		return where, fmt.Errorf(
			"expected Seek() to return %d, got %d",
			seek.ExpectedFinalPosition,
			where,
		)
	}

	return where, nil
}

// Factored out common code for reading
func checkStreamRead(
	stream *basicstream.BasicStream,
	seek SeekInfo,
	readSize int,
	rawUnderlyingBytes []byte,
	t *testing.T,
) {
	where, err := doCheckedSeek(stream, seek, t)
	if err != nil {
		t.Error(err.Error())
		return
	}

	expectedData := rawUnderlyingBytes[where : where+int64(readSize)]

	buffer := make([]byte, readSize)
	n, err := stream.Read(buffer)
	if err != nil {
		t.Errorf(
			"failed to read %d bytes from absolute offset %d: %s",
			readSize,
			where,
			err.Error(),
		)
	} else if n != readSize {
		t.Errorf(
			"wrong read size: expected %d bytes, got %d",
			readSize,
			n,
		)
	}

	if !bytes.Equal(expectedData, buffer) {
		t.Errorf(
			"%d bytes read at offset %d don't match expected data",
			n,
			where,
		)
	}
}

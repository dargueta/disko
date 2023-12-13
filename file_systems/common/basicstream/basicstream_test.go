package basicstream_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/file_systems/common/basicstream"
	diskotest "github.com/dargueta/disko/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "couldn't create stream")

	rawExpectedData, err := cache.Data()
	require.NoError(t, err, "failed to get cache data as slice")

	streamData := make([]byte, cache.Size())
	n, err := stream.Read(streamData)

	require.NoError(t, err, "failed to read entire stream contents")
	assert.EqualValues(t, cache.Size(), n, "read wrong number of bytes from stream")
	assert.True(
		t,
		bytes.Equal(rawExpectedData, streamData),
		"data read from stream is not equal to the expected raw data")
}

// Read less than one block (ensures rounding is correct)
func TestBasicStreamNew__LessThanOneBlock(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 16, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	require.NoError(t, err, "couldn't create stream")

	rawExpectedData, err := cache.GetSlice(0, 1)
	require.NoError(t, err, "failed to get cache data as slice")

	const readSize = 39

	streamData := make([]byte, readSize)
	n, err := stream.Read(streamData)
	require.NoErrorf(t, err, "failed to read %d bytes from stream", readSize)
	assert.EqualValues(t, 39, n, "read wrong number of bytes from stream")
	assert.True(
		t,
		bytes.Equal(rawExpectedData[:readSize], streamData),
		"data read from stream is not equal to the expected raw data")
}

// Read various sizes at various offsets from the beginning of the stream. This
// only tests Seek() with [io.SeekStart] as `whence`.
func TestBasicStream__SeekStart(t *testing.T) {
	cache := diskotest.CreateDefaultCache(128, 16, false, nil, t)
	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	require.NoError(t, err, "failed to create stream")

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
	require.NoError(t, err, "failed to create stream")

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
		doCheckedSeek(stream, seek, t)
	}
}

func TestBasicStream__ReadBasic(t *testing.T) {
	data := diskotest.CreateRandomImage(64, 8, t)
	require.Equal(t, 512, len(data), "raw data size is wrong")

	cache := diskotest.CreateDefaultCache(64, 8, false, data, t)
	require.EqualValues(t, 512, cache.Size(), "cache size is wrong")

	stream, err := basicstream.New(cache.Size(), cache, disko.O_RDONLY)
	require.NoError(t, err, "failed to create stream")
	defer stream.Close()

	assert.EqualValues(t, 0, stream.Tell(), "Tell() at beginning of stream should be 0")

	bytesRemaining := 512
	bytesRead := 0

	// If one byte is remaining then the read size will be 0 and we'll get stuck
	// in an infinite loop. Thus, we need to stop when there's only one byte left,
	// not 0.
	iterationNumber := 0
	for bytesRead < 511 {
		readSize := rand.Int() % bytesRemaining
		buffer := make([]byte, readSize)

		n, readErr := stream.Read(buffer)
		require.NoErrorf(
			t,
			readErr,
			"read failed (tried=%; remaining=%d; tell=%d; actual=%d; iteration=%d)",
			readSize,
			bytesRemaining,
			stream.Tell(),
			n,
			iterationNumber)

		assert.Equal(t, readSize, n, "read wrong # of bytes")
		assert.Equal(t, int64(bytesRead+n), stream.Tell(), "fpos is wrong")
		require.Truef(
			t,
			bytes.Equal(buffer, data[bytesRead:bytesRead+n]),
			"data read is wrong on iteration %d\nexpected: %v\ngot %v",
			iterationNumber,
			data[bytesRead:bytesRead+n],
			buffer)

		bytesRead += n
		bytesRemaining -= n
		iterationNumber++
	}

	assert.EqualValues(t, 511, bytesRead)
	assert.EqualValues(t, 1, bytesRemaining)
}

// doCheckedSeek performs the requested seek on the stream and checks the results.
// If the seek fails, it will return an error. The returned offset is always the
// return value of Seek(), even if it failed.
func doCheckedSeek(stream *basicstream.BasicStream, seek SeekInfo, t *testing.T) (int64, error) {
	originalAbsolutePosition := stream.Tell()
	where, err := stream.Seek(seek.Offset, seek.Whence)

	assert.NoErrorf(
		t,
		err,
		"failed to seek from %d to %d using offset %d, origin %d: %v",
		originalAbsolutePosition,
		seek.ExpectedFinalPosition,
		seek.Offset,
		seek.Whence,
		err)

	assert.Equal(
		t,
		seek.ExpectedFinalPosition,
		where,
		"return value of Seek() is wrong")

	assert.Equal(
		t,
		seek.ExpectedFinalPosition,
		stream.Tell(),
		"Tell() returned the wrong value")

	if err == nil && t.Failed() {
		return where, errors.New("one or more assertions failed")
	}
	return seek.ExpectedFinalPosition, err
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
	require.NoError(t, err)

	expectedData := rawUnderlyingBytes[where : where+int64(readSize)]

	buffer := make([]byte, readSize)
	n, err := stream.Read(buffer)
	assert.NoErrorf(
		t,
		err,
		"failed to read %d bytes from absolute offset %d: %s",
		readSize,
		where)
	assert.EqualValues(t, readSize, n, "read wrong number of bytes")
	assert.Truef(
		t,
		bytes.Equal(expectedData, buffer),
		"%d bytes read at offset %d don't match expected data",
		n,
		where)
}

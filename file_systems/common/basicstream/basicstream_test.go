package basicstream_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/file_systems/common/basicstream"
	diskotest "github.com/dargueta/disko/testing"
)

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
func TestBasicStreamNew__SeekStart(t *testing.T) {
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
		buffer := make([]byte, readSize)

		for _, offset := range byteOffsets {
			where, err := stream.Seek(offset, io.SeekStart)
			if err != nil {
				t.Errorf(
					"failed to seek to absolute offset %d: %s",
					offset,
					err.Error(),
				)
				continue
			} else if where != offset {
				t.Errorf(
					"expected Seek() to return %d, got %d",
					offset,
					where,
				)
			}

			expectedData := rawUnderlyingBytes[offset : offset+int64(readSize)]

			n, err := stream.Read(buffer)
			if err != nil {
				t.Errorf(
					"failed to read %d bytes from absolute offset %d: %s",
					readSize,
					offset,
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
					offset,
				)
			}
		}
	}
}

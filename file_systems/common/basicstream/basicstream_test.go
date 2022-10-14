package basicstream_test

import (
	"bytes"
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

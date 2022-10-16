package utilities

import (
	"bytes"
	"io"
	"testing"
)

func assertReadExpectedExactly(t *testing.T, raw, expected []byte) {
	buffer := bytes.NewBuffer(raw)
	reader, err := NewRLE90Reader(buffer)
	if err != nil {
		t.Errorf("failed to create reader: err should be nil")
	}

	output := make([]byte, len(expected))
	numRead, err := reader.Read(output)
	if err != nil {
		t.Errorf("failed to read data: err should be nil")
	}
	if numRead != len(expected) {
		t.Errorf("short read; expected %d, got %d", len(expected), numRead)
	}
	if !bytes.Equal(output, expected) {
		t.Errorf("data doesn't match: %v != expected %v", output, expected)
	}
}

type RawExpectedEntry struct {
	Name     string
	Raw      []byte
	Expected []byte
}

var BasicReadTests = [...]RawExpectedEntry{
	{Name: "NothingRepeated", Raw: []byte{0, 0x91, 0x23, 0x4f, 0}, Expected: []byte{0, 0x91, 0x23, 0x4f, 0}},
	{Name: "RepeatedNotCompressed", Raw: []byte{0xff, 0xff, 0xff}, Expected: []byte{0xff, 0xff, 0xff}},
	{Name: "Basic", Raw: []byte{0xff, 0x90, 0x05}, Expected: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	{Name: "BasicWithSurroundingData", Raw: []byte{0xe0, 0xff, 0x90, 0x05, 0x09}, Expected: []byte{0xe0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x09}},
	{Name: "EmptyOk", Raw: []byte{}, Expected: []byte{}},
	{Name: "ConsecutiveSame", Raw: []byte{0xe0, 0xff, 0x90, 0x02, 0x90, 0x03, 0x10}, Expected: []byte{0xe0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x10}},
	{Name: "ConsecutiveDifferent", Raw: []byte{0xe0, 0xff, 0x90, 0x03, 0x7a, 0x90, 0x04, 0x10}, Expected: []byte{0xe0, 0xff, 0xff, 0xff, 0xff, 0x7a, 0x7a, 0x7a, 0x7a, 0x7a, 0x10}},
	{Name: "Expand 0x90", Raw: []byte{0xe0, 0xff, 0x90, 0x05, 0x90, 0x00, 0x90, 0x02, 0xab}, Expected: []byte{0xe0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x90, 0x90, 0x90, 0xab}},
}

func TestRead__Basic(t *testing.T) {
	for _, test := range BasicReadTests {
		t.Run(test.Name, func(t *testing.T) { assertReadExpectedExactly(t, test.Raw, test.Expected) })
	}
}

func TestRead__ShortReadEmptyBuffer(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	reader, err := NewRLE90Reader(buffer)
	if err != nil {
		t.Errorf("failed to create reader: err should be nil")
	}

	output := make([]byte, 128)
	numRead, err := reader.Read(output)
	if err != io.EOF {
		t.Errorf("error should be io.EOF, got %v", err)
	}
	if numRead != 0 {
		t.Errorf("number of bytes read should be 0, got %d", numRead)
	}
}

package compression_test

import (
	"bytes"
	"io"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
)

type BasicTestCase struct {
	Data           []byte
	ExpectedResult c.ByteRun
	Name           string
}

var basicTestCases = []BasicTestCase{
	{[]byte{}, c.InvalidRLERun, "empty"},
	{[]byte{0, 0, 1, 0, 0, 0, 0}, c.ByteRun{Byte: byte(0), RunLength: 2}, "two initial"},
	{[]byte{6, 1, 5, 20, 31}, c.ByteRun{Byte: byte(6), RunLength: 1}, "one byte"},
	{[]byte{9, 9, 9, 9, 9, 9}, c.ByteRun{Byte: byte(9), RunLength: 6}, "entire run"},
}

func runBasicTestCase(t *testing.T, test BasicTestCase) {
	grouper := c.NewRLEGrouper(bytes.NewBuffer(test.Data))
	result, _ := grouper.GetNextRun()
	if result != test.ExpectedResult {
		t.Errorf("Expected %+v, got %+v", test.ExpectedResult, result)
	}
}

func TestRLEGrouper__Basic(t *testing.T) {
	for _, test := range basicTestCases {
		t.Run(
			test.Name,
			func(t *testing.T) {
				runBasicTestCase(t, test)
			},
		)
	}
}

func TestRLEGrouper__Sequence(t *testing.T) {
	data := []byte{1, 9, 4, 4, 4, 4, 4, 6, 6, 0, 1, 0, 0, 0}
	expected := []c.ByteRun{
		{byte(1), 1}, {byte(9), 1}, {byte(4), 5}, {byte(6), 2}, {byte(0), 1},
		{byte(1), 1}, {byte(0), 3}, c.InvalidRLERun,
	}

	buffer := bytes.NewBuffer(data)
	grouper := c.NewRLEGrouper(buffer)
	for i, expectedRun := range expected {
		result, err := grouper.GetNextRun()
		if result != expectedRun {
			t.Errorf(
				"run %d is wrong: expected %+v but got %+v",
				i,
				expectedRun,
				result,
			)
		}
		if expectedRun == c.InvalidRLERun && err != io.EOF {
			t.Errorf("expected err to be io.EOF, got %q", err.Error())
		}
	}
}

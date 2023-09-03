package compression_test

import (
	"bytes"
	"io"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, test.ExpectedResult, result)
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
	hitEOF := false

	for i, expectedRun := range expected {
		result, err := grouper.GetNextRun()
		assert.Equalf(t, expectedRun, result, "run %d is wrong", i)
		if expectedRun == c.InvalidRLERun {
			assert.Equal(t, io.EOF, err, "expected io.EOF sentinel error")
			hitEOF = true
		}
	}
	assert.True(t, hitEOF, "never hit EOF sentinel")
}

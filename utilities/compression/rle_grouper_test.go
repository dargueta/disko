package compression_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	c "github.com/dargueta/disko/utilities/compression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A FailingReader is an [io.Reader] that returns a user-supplied error once the
// given data (if any) has been exhausted.
type FailingReader struct {
	Data  io.Reader
	Error error
}

// Read implements [io.Reader].
func (fr FailingReader) Read(buf []byte) (int, error) {
	n, err := fr.Data.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		panic(
			fmt.Errorf(
				"unexpected error getting %d bytes in FailingReader: %w",
				len(buf),
				err))
	}

	if n > 0 {
		return n, err
	}
	// n == 0, input has been exhausted. We can now return the user's desired error.
	return 0, fr.Error
}

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

func TestRLEGrouper__ErrorOnFirstRead(t *testing.T) {
	expectedError := errors.New("this is the expected error")
	reader := FailingReader{Data: &bytes.Buffer{}, Error: expectedError}

	grouper := c.NewRLEGrouper(reader)
	result, err := grouper.GetNextRun()

	assert.ErrorIs(t, err, expectedError)
	assert.Equal(t, c.InvalidRLERun, result)
}

func TestRLEGrouper__ErrorOnSubsequent(t *testing.T) {
	expectedError := errors.New("this is the expected error")
	reader := FailingReader{
		Data:  bytes.NewBuffer([]byte{1, 1, 1, 2}),
		Error: expectedError,
	}

	grouper := c.NewRLEGrouper(reader)

	// First run
	result, err := grouper.GetNextRun()
	assert.Equal(t, byte(1), result.Byte, "byte is wrong for run 1")
	assert.Equal(t, 3, result.RunLength, "run length is wrong for run 1")
	require.NoError(t, err, "run 1 failed")

	// Second run
	result, err = grouper.GetNextRun()
	assert.Equal(t, byte(2), result.Byte, "byte is wrong for run 2")
	assert.Equal(t, 1, result.RunLength, "run length is wrong for run 2")
	require.NoError(t, err, "run 2 failed")

	// Third run should fail
	result, err = grouper.GetNextRun()
	assert.ErrorIs(t, err, expectedError)
	assert.Equal(t, c.InvalidRLERun, result)
}

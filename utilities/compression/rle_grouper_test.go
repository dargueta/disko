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

// A FailingReader is an [io.ByteScanner] that returns a user-supplied error once
// the given data (if any) has been exhausted.
type FailingReader struct {
	Data  io.ByteScanner
	Error error
	T     *testing.T
}

// ReadByte implements [io.ByteScanner].
func (fr FailingReader) ReadByte() (byte, error) {
	fr.T.Helper()

	byteVal, err := fr.Data.ReadByte()
	if err == nil {
		return byteVal, nil
	}

	if errors.Is(err, io.EOF) {
		// Input has been exhausted. We can now return the user's error.
		return 0, fr.Error
	}

	// An error occurred and it's not EOF.
	panic(
		fmt.Errorf("unexpected error getting byte in FailingReader: %w", err))
}

// UnreadByte implements [io.ByteScanner].
func (fr FailingReader) UnreadByte() error {
	fr.T.Helper()
	return fr.Data.UnreadByte()
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
	grouper := c.NewRLEGrouperFromByteScanner(bytes.NewBuffer(test.Data))
	result, _ := grouper.GetNextRun()
	assert.Equal(t, test.ExpectedResult, result)
}

func TestRLEGrouper__Basic(t *testing.T) {
	for _, test := range basicTestCases {
		t.Run(
			test.Name,
			func(t *testing.T) { runBasicTestCase(t, test) },
		)
	}
}

type FullTestCase struct {
	Name         string
	RawBytes     []byte
	ExpectedRuns []c.ByteRun
}

var fullTestCases = []FullTestCase{
	{
		"empty",
		[]byte{},
		[]c.ByteRun{c.InvalidRLERun},
	},
	{
		"basic",
		[]byte{1, 9, 4, 4, 4, 4, 4, 6, 6, 0, 1, 0, 0, 0},
		[]c.ByteRun{
			{byte(1), 1}, {byte(9), 1}, {byte(4), 5}, {byte(6), 2}, {byte(0), 1},
			{byte(1), 1}, {byte(0), 3}, c.InvalidRLERun,
		},
	},
	{
		"leading run",
		[]byte{1, 1, 1, 127},
		[]c.ByteRun{{byte(1), 3}, {byte(127), 1}, c.InvalidRLERun},
	},
	{
		"trailing run",
		[]byte{127, 127, 1, 1, 1},
		[]c.ByteRun{{byte(127), 2}, {byte(1), 3}, c.InvalidRLERun},
	},
	{
		"trailing run with single after",
		[]byte{127, 127, 1, 1, 1, 1, 3},
		[]c.ByteRun{{byte(127), 2}, {byte(1), 4}, {byte(3), 1}, c.InvalidRLERun},
	},
}

func runFullRunTestCase(t *testing.T, testCase *FullTestCase) {
	t.Helper()

	buffer := bytes.NewBuffer(testCase.RawBytes)
	grouper := c.NewRLEGrouperFromByteScanner(buffer)
	hitEOF := false

	for i, expectedRun := range testCase.ExpectedRuns {
		require.Falsef(t, hitEOF, "grouper hit EOF early, on run %d", i)

		result, err := grouper.GetNextRun()
		assert.Equalf(t, expectedRun, result, "run %d is wrong", i)

		if expectedRun == c.InvalidRLERun {
			assert.ErrorIs(t, err, io.EOF, "expected io.EOF sentinel error")
			hitEOF = true
		}
	}
	assert.True(t, hitEOF, "never hit EOF sentinel")
}

func TestRLEGrouper__FullInputs(t *testing.T) {
	for _, testCase := range fullTestCases {
		t.Run(
			testCase.Name,
			func(subT *testing.T) { runFullRunTestCase(t, &testCase) },
		)
	}
}

func TestRLEGrouper__ErrorOnFirstRead(t *testing.T) {
	expectedError := errors.New("this is the expected error")
	reader := FailingReader{Data: &bytes.Buffer{}, Error: expectedError, T: t}

	grouper := c.NewRLEGrouperFromByteScanner(reader)
	result, err := grouper.GetNextRun()

	assert.ErrorIs(t, err, expectedError)
	assert.Equal(t, c.InvalidRLERun, result)
}

func TestRLEGrouper__ErrorAfterLastRun(t *testing.T) {
	expectedError := errors.New("this is the expected error")
	reader := FailingReader{
		Data:  bytes.NewBuffer([]byte{1, 1, 1, 2, 2, 3}),
		Error: expectedError,
		T:     t,
	}

	grouper := c.NewRLEGrouperFromByteScanner(reader)

	// First run
	t.Log("Reading run 1")
	result, err := grouper.GetNextRun()
	assert.Equal(t, byte(1), result.Byte, "byte is wrong for run 1")
	assert.Equal(t, 3, result.RunLength, "run length is wrong for run 1")
	require.NoError(t, err, "run 1 failed")

	// Second run
	t.Log("Reading run 2")
	result, err = grouper.GetNextRun()
	assert.Equal(t, byte(2), result.Byte, "byte is wrong for run 2")
	assert.Equal(t, 2, result.RunLength, "run length is wrong for run 2")
	require.NoError(t, err, "run 2 failed")

	// Third run should fail
	t.Log("Reading run 3")
	result, err = grouper.GetNextRun()
	assert.ErrorIs(t, err, expectedError)
	assert.Equal(t, c.InvalidRLERun, result)
}

func TestRLEGrouper__ErrorWhileReadingARun(t *testing.T) {
	expectedError := errors.New("this is the expected error")
	reader := FailingReader{
		Data:  bytes.NewBuffer([]byte{1, 1, 1, 2, 2}),
		Error: expectedError,
		T:     t,
	}

	grouper := c.NewRLEGrouperFromByteScanner(reader)

	// First run
	t.Log("Reading run 1")
	result, err := grouper.GetNextRun()
	assert.Equal(t, byte(1), result.Byte, "byte is wrong for run 1")
	assert.Equal(t, 3, result.RunLength, "run length is wrong for run 1")
	require.NoError(t, err, "run 1 failed")

	// Second run should fail
	t.Log("Reading run 2")
	result, err = grouper.GetNextRun()
	assert.ErrorIs(t, err, expectedError, "run 2 succeeded unexpectedly")
}

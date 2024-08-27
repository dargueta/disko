package fat8

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGeometryInvalidBlocks(t *testing.T) {
	_, err := GetGeometry(719)
	assert.Error(t, err)
}

func TestGetGeometryAliasing(t *testing.T) {
	big, _ := GetGeometry(2002)
	small, _ := GetGeometry(1898)

	assert.EqualValues(
		t, 77, big.TrueTotalTracks, "2002-sector image has wrong # tracks")
	assert.EqualValues(
		t, 73, small.TrueTotalTracks, "1898-sector image has wrong # tracks")
	assert.LessOrEqual(t, big.TotalTracks, big.TrueTotalTracks)

	// TrueTotalTracks have the expected values so we'll overwrite the fields we
	// don't care about.
	big.TrueTotalTracks = 0
	small.TrueTotalTracks = 0
	assert.Equal(t, big, small, "geometry should match")
}

type FilenameTest struct {
	Filename   string
	BinaryForm []byte
}

var filenameTests = [...]FilenameTest{
	{Filename: "qwerty.txt", BinaryForm: []byte("QWERTYTXT")},
	{Filename: "aSdF.g", BinaryForm: []byte("ASDF  G  ")},
	{Filename: "noext", BinaryForm: []byte("NOEXT    ")},
	{Filename: "a B.C", BinaryForm: []byte("A B   C  ")},
}

func TestSerializeFilenames(t *testing.T) {
	for _, test := range filenameTests {
		serialized, err := FilenameToBytes(test.Filename)
		assert.NoErrorf(t, err, "serializing %q failed", test.Filename)
		assert.Truef(t, bytes.Equal(serialized, test.BinaryForm),
			"Serialized filename is wrong; expected %q, got %q",
			test.BinaryForm,
			serialized,
		)
	}
}

func TestDeserializeFilenames(t *testing.T) {
	for _, test := range filenameTests {
		deserialized, err := BytesToFilename(test.BinaryForm)
		assert.NoErrorf(t, err, "deserializing %q failed", test.Filename)
		assert.Truef(t, strings.EqualFold(deserialized, test.Filename),
			"Serialized filename is wrong; expected %q, got %q",
			strings.ToUpper(test.Filename),
			deserialized,
		)
	}
}

func TestEmptyFilenameBad(t *testing.T) {
	_, err := FilenameToBytes("")
	assert.Error(t, err)
}

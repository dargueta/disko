package fat8

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetGeometryInvalidBlocks(t *testing.T) {
	_, err := GetGeometry(719)
	if err == nil {
		t.Errorf("GetGeometry didn't fail on an invalid number of blocks.")
	}
}

func TestGetGeometryAliasing(t *testing.T) {
	big, _ := GetGeometry(2002)
	small, _ := GetGeometry(1898)

	if big != small {
		t.Errorf("Geometry for 2002 != 1898")
	}

	if big.TrueTotalTracks <= big.TotalTracks {
		t.Errorf("Condition failed: %d > %d", big.TrueTotalTracks, big.TotalTracks)
	}
}

type FilenameTest struct {
	Filename   string
	BinaryForm []byte
}

var filenameTests = [...]FilenameTest{
	{Filename: "", BinaryForm: []byte("         ")},
	{Filename: "qwerty.txt", BinaryForm: []byte("QWERTYTXT")},
	{Filename: "aSdF.g", BinaryForm: []byte("ASDF  G  ")},
	{Filename: "noext", BinaryForm: []byte("NOEXT    ")},
	{Filename: "a B.C", BinaryForm: []byte("A B   C  ")},
}

func TestSerializeFilenames(t *testing.T) {
	for _, test := range filenameTests {
		serialized, err := FilenameToBytes(test.Filename)
		if err != nil {
			t.Errorf("Error serializing `%s`: %s", test.Filename, err.Error())
		} else if !bytes.Equal(serialized, test.BinaryForm) {
			t.Errorf(
				"Serialized filename is wrong; expected `%s`, got `%s`",
				test.BinaryForm,
				serialized,
			)
		}
	}
}

func TestDeserializeFilenames(t *testing.T) {
	for _, test := range filenameTests {
		deserialized, err := BytesToFilename(test.BinaryForm)
		if err != nil {
			t.Errorf("Error deserializing `%s`: %s", test.Filename, err.Error())
		} else if !strings.EqualFold(deserialized, test.Filename) {
			t.Errorf(
				"Serialized filename is wrong; expected `%s`, got `%s`",
				strings.ToUpper(test.Filename),
				deserialized,
			)
		}
	}
}

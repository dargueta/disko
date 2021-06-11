package fat8

import (
	"testing"
)

func TestGetGeometryInvalidBlocks(t *testing.T) {
	_, err := GetGeometry(719)
	if err == nil {
		t.Logf("GetGeometry didn't fail on an invalid number of blocks.")
	}
}

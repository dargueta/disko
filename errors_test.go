package disko_test

import (
	"errors"
	"testing"

	"github.com/dargueta/disko"
	"github.com/stretchr/testify/assert"
)

func TestDiskoErrorWithMessage(t *testing.T) {
	newErr := disko.ErrBlockDeviceRequired.WithMessage("asdfqwerty")
	assert.Equal(
		t, "Block device required: asdfqwerty", newErr.Error(), "error message is wrong")
	assert.ErrorIs(t, newErr, disko.ErrBlockDeviceRequired)
}

func TestDiskoErrorWrap(t *testing.T) {
	originalErr := errors.New("original error")
	newErr := disko.ErrExists.Wrap(originalErr)
	expectedMessage := "File exists: original error"

	assert.EqualValues(t, expectedMessage, newErr.Error(), "error message is wrong")
	assert.ErrorIs(t, newErr, originalErr, "original error not set as parent")
	assert.ErrorIs(t, newErr, disko.ErrExists, "Disko error not set as parent")
}

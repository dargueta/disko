package disko_test

import (
	"errors"
	"testing"

	"github.com/dargueta/disko"
)

func TestDiskoErrorWithMessage(t *testing.T) {
	newErr := disko.ErrBlockDeviceRequired.WithMessage("asdfqwerty")
	expectedMessage := "Block device required: asdfqwerty"

	if newErr.Error() != expectedMessage {
		t.Errorf(
			"error message is wrong: expected %q, got %q",
			expectedMessage,
			newErr.Error(),
		)
	}

	if !errors.Is(newErr, disko.ErrBlockDeviceRequired) {
		t.Error("error is wrapped improperly, Disko error not set as parent")
	}
}

func TestDiskoErrorWrap(t *testing.T) {
	originalErr := errors.New("original error")
	newErr := disko.ErrExists.Wrap(originalErr)
	expectedMessage := "File exists: original error"

	if newErr.Error() != expectedMessage {
		t.Errorf(
			"error message is wrong: expected %q, got %q",
			expectedMessage,
			newErr.Error(),
		)
	}

	if !errors.Is(newErr, originalErr) {
		t.Error("error is wrapped improperly, original error not set as parent")
	}
	if !errors.Is(newErr, disko.ErrExists) {
		t.Error("error is wrapped improperly, Disko error not set as parent")
	}
}

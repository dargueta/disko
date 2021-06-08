package fat8

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/dargueta/disko"
)

// FilenameToBytes converts a filename string to its on-disk representation. The
// returned name will be normalized to uppercase.
// TODO(dargueta): Ensure the filename has no invalid characters.
func FilenameToBytes(name string) ([]byte, error) {
	parts := strings.SplitN(name, ".", 1)

	// Unless we got an empty string, this will always have at least one element,
	// the stem of the filename. This cannot be longer than 6 characters.
	if len(parts[0]) > 6 {
		message := fmt.Sprintf(
			"filename stem can be at most six characters: `%s`", parts[0])
		return nil, disko.NewDriverErrorWithMessage(disko.ENAMETOOLONG, message)
	}

	var paddedName string
	// If there are two parts to the filename then there's an extension (probably)
	if len(parts) == 2 {
		if len(parts[1]) == 0 {
			// Second part after the period is empty, which means the filename
			// ended with a period. This is stupid, but not prohibited by the
			// standard (I think) so we must support it.
			parts[1] = "."
		} else if len(parts[1]) > 3 {
			// Extension is longer than three characters.
			message := fmt.Sprintf(
				"filename extension can be at most three characters: `%s`", parts[1])
			return nil, disko.NewDriverErrorWithMessage(disko.ENAMETOOLONG, message)
		}

		paddedName = fmt.Sprintf("%-6s%-3s", parts[0], parts[1])
	} else {
		// Filename has no extension.
		paddedName = fmt.Sprintf("%-9s", parts[0])
	}

	return []byte(strings.ToUpper(paddedName)), nil
}

// BytesToFilename converts the on-disk representation of a filename into its
// user-friendly form.
func BytesToFilename(rawName []byte) string {
	stem := bytes.TrimRight(rawName[:6], " ")
	extension := bytes.TrimRight(rawName[6:], " ")

	var name string
	if len(extension) > 0 {
		name = string(stem) + "." + string(extension)
	} else {
		name = string(stem)
	}
	return strings.ToUpper(name)
}

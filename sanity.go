package archiver

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ExtractPathFunc represent a func to eval extract path for an entry in compressed file
type ExtractPathFunc func(filePath string, destination string) (string, error)

// SanitizeExtractPathFunc is fail fast func meeting zip slip situation
var zipSlipExtractPathFunc = func(filePath string, destination string) (string, error) {
	if err := zipSlipExtractPath(filePath, destination); err != nil {
		return "", err
	}
	return filepath.Join(destination, filePath), nil
}

// DefaultExtractPathFunc is used by zip, rar, tar. Modify it to different behavior
var DefaultExtractPathFunc = zipSlipExtractPathFunc

func zipSlipExtractPath(filePath string, destination string) error {
	// to avoid zip slip (writing outside of the destination), we resolve
	// the target path, and make sure it's nested in the intended
	// destination, or bail otherwise.
	destpath := filepath.Join(destination, filePath)
	if !strings.HasPrefix(destpath, filepath.Clean(destination)) {
		return fmt.Errorf("%s: illegal file path", filePath)
	}
	return nil
}

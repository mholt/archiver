package archiver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
)

// Rar makes a .rar archive, but this is not implemented because
// RAR is a proprietary format. It is here only for symmetry with
// the other archive formats in this package.
func Rar(rarPath string, filePaths []string) error {
	return fmt.Errorf("make %s: RAR not implemented (proprietary format)", rarPath)
}

// Unrar extracts the RAR file at source and puts the contents
// into destination.
func Unrar(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	rr, err := rardecode.NewReader(f, "")
	if err != nil {
		return fmt.Errorf("%s: failed to create reader: %v", source, err)
	}

	// we should only parse for subfolders once per unrar request
	subfoldersCreated := false

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// make subfolders for this path
		if !subfoldersCreated {
			makeSubfolders(filepath.Dir(header.Name), destination)
			subfoldersCreated = true
		}

		if header.IsDir {
			err = mkdir(filepath.Join(destination, header.Name))
			if err != nil {
				return err
			}
			continue
		}

		err = writeNewFile(filepath.Join(destination, header.Name), rr, header.Mode())
		if err != nil {
			return err
		}
	}

	return nil
}

// makeSubfolders will parse a path string for subfolders
// and create them as needed.
func makeSubfolders(path string, destination string) {
	// parse path for subfolders
	for _, subfolder := range strings.Split(path, "/") {
		filepath.Dir(subfolder)

		// check to see if the subfolder exists already
		if stat, err := os.Stat(destination + subfolder); err != nil || !stat.IsDir() {
			// make the directory
			mkdir(destination + subfolder)
			continue
		}
	}
}

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

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		
		pathComponents := strings.Split(header.Name, "/")

		for pi, path := range pathComponents {
			// the last component of the path will be the file
			// so ignore it, since we only want to create folders
			if pi == len(pathComponents)-1 {
				continue
			}

			// check to see if the path exists already
			if stat, err := os.Stat(destination + path); err != nil || !stat.IsDir() {
				// make the directory
				mkdir(destination + path)
				continue
			}
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

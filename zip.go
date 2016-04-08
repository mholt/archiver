// Package archiver makes it super easy to create .zip and .tar.gz archives.
package archiver

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Zip creates a .zip file in the location zipPath containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func Zip(zipPath string, filePaths []string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", zipPath, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	for _, fpath := range filePaths {
		err = zipFile(w, fpath)
		if err != nil {
			w.Close()
			return err
		}
	}

	return w.Close()
}

func zipFile(w *zip.Writer, source string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("error stat'ing %s: %v", source, err)
	}

	var baseDir string
	if sourceInfo.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("error making header for %s: %v", path, err)
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += string(filepath.Separator)
		} else {
			header.Method = zip.Deflate
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("error making header for %s: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening %s: %v", path, err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("error copying contents of %s: %v", path, err)
		}

		return nil
	})
}

package archiver

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TarGz creates a .tar.gz file at targzPath containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func TarGz(targzPath string, filePaths []string) error {
	out, err := os.Create(targzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", targzPath, err)
	}
	defer out.Close()

	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, fpath := range filePaths {
		err := tarGzFile(tarWriter, fpath)
		if err != nil {
			return err
		}
	}

	return nil
}

func tarGzFile(tarWriter *tar.Writer, source string) error {
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

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("error making header for %s: %v", path, err)
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("error writing header for %s: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening %s: %v", path, err)
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return fmt.Errorf("error copying contents of %s: %v", path, err)
		}

		return nil
	})
}

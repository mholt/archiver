package archiver

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
)

// TarGz is for TarGz format
var TarGz tarGzFormat

func init() {
	RegisterFormat("TarGz", TarGz)
}

type tarGzFormat struct{}

func (tarGzFormat) Match(filename string) bool {
	// TODO: read file header to identify the format
	return strings.HasSuffix(strings.ToLower(filename), ".tar.gz") ||
		strings.HasSuffix(strings.ToLower(filename), ".tgz")
}

// Make creates a .tar.gz file at targzPath containing
// the contents of files listed in filePaths. It works
// the same way Tar does, but with gzip compression.
func (tarGzFormat) Make(targzPath string, filePaths []string) error {
	out, err := os.Create(targzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", targzPath, err)
	}
	defer out.Close()

	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, targzPath)
}

// Open untars source and decompresses the contents into destination.
func (tarGzFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: create new gzip reader: %v", source, err)
	}
	defer gzr.Close()

	return untar(tar.NewReader(gzr), destination)
}

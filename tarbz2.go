package archiver

import (
	"archive/tar"
	"fmt"
	"os"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

func init() {
	RegisterFormat("TarBz2", tarBz2Format{})
}

type tarBz2Format struct{}

func (tarBz2Format) Match(filename string) bool {
	// TODO: read file header to identify the format
	return strings.HasSuffix(strings.ToLower(filename), ".tar.bz2")
}

// Make creates a .tar.bz2 file at tarbz2Path containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarBz2Format) Make(tarbz2Path string, filePaths []string) error {
	out, err := os.Create(tarbz2Path)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarbz2Path, err)
	}
	defer out.Close()

	bz2Writer, err := bzip2.NewWriter(out, nil)
	if err != nil {
		return fmt.Errorf("error compressing %s: %v", tarbz2Path, err)
	}
	defer bz2Writer.Close()

	tarWriter := tar.NewWriter(bz2Writer)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, tarbz2Path)
}

// Open untars source and decompresses the contents into destination.
func (tarBz2Format) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	bz2r, err := bzip2.NewReader(f, nil)
	if err != nil {
		return fmt.Errorf("error decompressing %s: %v", source, err)
	}
	defer bz2r.Close()

	return untar(tar.NewReader(bz2r), destination)
}

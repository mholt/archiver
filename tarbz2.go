package archiver

import (
	"archive/tar"
	"fmt"
	"os"

	"github.com/dsnet/compress/bzip2"
)

// TarBz2 creates a .tar.bz2 file at tarbz2Path containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func TarBz2(tarbz2Path string, filePaths []string) error {
	out, err := os.Create(tarbz2Path)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarbz2Path, err)
	}
	defer out.Close()

	bz2Writer := bzip2.NewWriter(out)
	defer bz2Writer.Close()

	tarWriter := tar.NewWriter(bz2Writer)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, tarbz2Path)
}

// UntarBz2 untars source and decompresses the contents into destination.
func UntarBz2(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	bz2r := bzip2.NewReader(f)
	defer bz2r.Close()

	return untar(tar.NewReader(bz2r), destination)
}

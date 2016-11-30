package archiver

import (
	"archive/tar"
	"fmt"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

// TarXZ is for TarXZ format
var TarXZ xzFormat

func init() {
	RegisterFormat("TarXZ", TarXZ)
}

type xzFormat struct{}

// Match returns whether filename matches this format.
func (xzFormat) Match(filename string) bool {
	// TODO: read file header to identify the format
	return strings.HasSuffix(strings.ToLower(filename), ".tar.xz") ||
		strings.HasSuffix(strings.ToLower(filename), ".txz")
}

// Make creates a .tar.xz file at xzPath containing
// the contents of files listed in filePaths. File
// paths can be those of regular files or directories.
// Regular files are stored at the 'root' of the
// archive, and directories are recursively added.
func (xzFormat) Make(xzPath string, filePaths []string) error {
	out, err := os.Create(xzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", xzPath, err)
	}
	defer out.Close()

	xzWriter, err := xz.NewWriter(out)
	if err != nil {
		return fmt.Errorf("error compressing %s: %v", xzPath, err)
	}
	defer xzWriter.Close()

	tarWriter := tar.NewWriter(xzWriter)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, xzPath)
}

// Open untars source and decompresses the contents into destination.
func (xzFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	xzReader, err := xz.NewReader(f)
	if err != nil {
		return fmt.Errorf("error decompressing %s: %v", source, err)
	}

	return untar(tar.NewReader(xzReader), destination)
}

package archiver

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

// Gz is for Gz format
var Gz gzFormat

func init() {
	RegisterFormat("Gz", Gz)
}

type gzFormat struct{}

func (gzFormat) Match(filename string) bool {
	return (strings.HasSuffix(strings.ToLower(filename), ".gz") &&
		!strings.HasSuffix(strings.ToLower(filename), ".tar.gz") &&
		!strings.HasSuffix(strings.ToLower(filename), ".tgz")) ||
		(!isTarGz(filename) &&
			isGz(filename))
}

// isGz checks if the file is a valid gzip.
func isGz(gzPath string) bool {
	f, err := os.Open(gzPath)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = gzip.NewReader(f)
	if err == gzip.ErrHeader {
		return false
	}

	return true
}

// Write outputs to a Writer the gzip'd contents of the first file listed in
// filePaths.
func (gzFormat) Write(output io.Writer, filePaths []string) error {
	return writeGz(filePaths, output, "")
}

// Make creates a file at gzPath containing the gzip'd contents of the first file
// listed in filePaths.
func (gzFormat) Make(gzPath string, filePaths []string) error {
	out, err := os.Create(gzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", gzPath, err)
	}
	defer out.Close()

	return writeGz(filePaths, out, gzPath)
}

func writeGz(filePaths []string, output io.Writer, dest string) error {
	if len(filePaths) != 1 {
		return fmt.Errorf("only one file supported for gz")
	}
	firstFile := filePaths[0]

	fileInfo, err := os.Stat(firstFile)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", firstFile, err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("%s is a directory", firstFile)
	}

	in, err := os.Open(firstFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", firstFile, err)
	}
	defer in.Close()

	gzw := gzip.NewWriter(output)
	defer gzw.Close()

	if _, err = io.Copy(gzw, in); err != nil {
		return fmt.Errorf("error writing gz: %v", err)
	}
	return nil
}

// Read a gzip'd file from a Reader and decompresses the contents into
// destination.
func (gzFormat) Read(input io.Reader, destination string) error {
	gzr, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("error decompressing: %v", err)
	}
	defer gzr.Close()

	return writeNewFile(destination, gzr, 0644)
}

// Open decompresses gzip'd source into destination.
func (gzFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return Gz.Read(f, destination)
}

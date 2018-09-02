package archiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// Bzip2 is for Bzip2 format
var Bzip2 bzip2Format

func init() {
	RegisterFormat("Bzip2", Bzip2)
}

type bzip2Format struct{}

func (bzip2Format) Match(filename string) bool {
	return (strings.HasSuffix(strings.ToLower(filename), ".bz2") &&
		!strings.HasSuffix(strings.ToLower(filename), ".tar.bz2") &&
		!strings.HasSuffix(strings.ToLower(filename), ".tbz2")) ||
		(!isTarBz2(filename) &&
			isBz2(filename))
}

// isBz2 checks if the file is a valid bzip2.
func isBz2(bzip2Path string) bool {
	f, err := os.Open(bzip2Path)
	if err != nil {
		return false
	}
	defer f.Close()

	bzip2r, err := bzip2.NewReader(f, nil)
	if err != nil {
		return false
	}

	buf := make([]byte, 16)
	if _, err = io.ReadFull(bzip2r, buf); err != nil {
		return false
	}

	return true
}

// Write outputs to a Writer the bzip2'd contents of the first file listed in
// filePaths.
func (bzip2Format) Write(output io.Writer, filePaths []string) error {
	return writeBzip2(filePaths, output, "")
}

// Make creates a file at bzip2Path containing the bzip2'd contents of the first file
// listed in filePaths.
func (bzip2Format) Make(bzip2Path string, filePaths []string) error {
	out, err := os.Create(bzip2Path)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", bzip2Path, err)
	}
	defer out.Close()

	return writeBzip2(filePaths, out, bzip2Path)
}

func writeBzip2(filePaths []string, output io.Writer, dest string) error {
	if len(filePaths) != 1 {
		return fmt.Errorf("only one file supported for bzip2")
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

	bzip2w, err := bzip2.NewWriter(output, nil)
	if err != nil {
		return fmt.Errorf("error compressing bzip2: %v", err)
	}
	defer bzip2w.Close()

	if _, err = io.Copy(bzip2w, in); err != nil {
		return fmt.Errorf("error writing bzip2: %v", err)
	}
	return nil
}

// Read a bzip2'd file from a Reader and decompresses the contents into
// destination.
func (bzip2Format) Read(input io.Reader, destination string) error {
	bzip2r, err := bzip2.NewReader(input, nil)
	if err != nil {
		return fmt.Errorf("error decompressing: %v", err)
	}
	defer bzip2r.Close()

	return writeNewFile(destination, bzip2r, 0644)
}

// Open decompresses bzip2'd source into destination.
func (bzip2Format) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return Bzip2.Read(f, destination)
}

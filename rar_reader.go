package archiver

import (
	"github.com/nwaples/rardecode"
	"io"
	"fmt"
	"errors"
	"os"
)
var RarReader rarFormatReader

type rarFormat struct{}

type rarFormatEntry struct {
	rarReader *rardecode.Reader
	header    *rardecode.FileHeader
}

func (entry rarFormatEntry) Name() string {
	if entry.header != nil {
		return entry.header.Name
	}
	return ""
}

func (entry rarFormatEntry) IsDirectory() bool {
	if entry.header != nil {
		return entry.header.IsDir
	}
	return false
}

func (entry *rarFormatEntry) Write(output io.Writer) error {
	if entry.rarReader == nil {
		return errors.New("no Reader")
	}
	_, err := io.Copy(output, entry.rarReader)
	return err
}

type rarFormatReader struct {
	rarReader *rardecode.Reader
}

func (rfr *rarFormatReader) Close() error {
	return nil
}

func (rfr *rarFormatReader) OpenPath(path string) error {
	rf, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %v", path, err)
	}

	return rfr.Open(rf)
}

func (rfr *rarFormatReader) Open(input io.Reader) error {
	var err error
	rfr.rarReader, err = rardecode.NewReader(input, "")
	if err != nil {
		return fmt.Errorf("read: failed to create reader: %v", err)
	}
	return nil
}

// Read extracts the RAR file read from input and puts the contents
// into destination.
func (rfr *rarFormatReader) ReadEntry() (Entry, error) {
	header, err := rfr.rarReader.Next()
	if err == io.EOF {
		return NilEntry, nil
	} else if err != nil {
		return NilEntry, err
	}

	return &rarFormatEntry{
		rfr.rarReader,
		header}, nil
}
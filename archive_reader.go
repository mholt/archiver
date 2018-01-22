package archiver

import (
	"io"
	"errors"
	"os"
)

type ArchiveReader interface {
	OpenPath(path string) error
	Open(io.Reader) error
	ReadEntry() (Entry, error)
	Close() error
}

type Entry interface {
	Name() string
	IsDirectory() bool
	Mode() os.FileMode
	Write(output io.Writer) error
}

type nilEntry struct {
}

func (entry nilEntry) Name() string {
	return "nil"
}

func (entry nilEntry) IsDirectory() bool {
	return false
}

func (entry nilEntry) Mode() os.FileMode {
	return os.ModeAppend
}

func (entry nilEntry) Write(output io.Writer) error {
	return errors.New("nil")
}

var NilEntry = nilEntry{}
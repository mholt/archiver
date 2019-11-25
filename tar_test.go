package archiver_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/mholt/archiver/v3"
)

func requireRegularFile(t *testing.T, path string) os.FileInfo {
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("fileInfo on '%s': %v", path, err)
	}

	if !fileInfo.Mode().IsRegular() {
		t.Fatalf("'%s' expected to be a regular file", path)
	}

	return fileInfo
}

func assertSameFile(t *testing.T, f1, f2 os.FileInfo) {
	if !os.SameFile(f1, f2) {
		t.Errorf("expected '%s' and '%s' to be the same file", f1.Name(), f2.Name())
	}
}

func TestDefaultTar_Unarchive_HardlinkSuccess(t *testing.T) {
	source := "testdata/gnu-hardlinks.tar"

	destination, err := ioutil.TempDir("", "archiver_tar_test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(destination)

	err = archiver.DefaultTar.Unarchive(source, destination)
	if err != nil {
		t.Fatalf("unarchiving '%s' to '%s': %v", source, destination, err)
	}

	fileaInfo := requireRegularFile(t, path.Join(destination, "dir-1", "dir-2", "file-a"))
	filebInfo := requireRegularFile(t, path.Join(destination, "dir-1", "dir-2", "file-b"))
	assertSameFile(t, fileaInfo, filebInfo)
}

func TestDefaultTar_Extract_HardlinkSuccess(t *testing.T) {
	source := "testdata/gnu-hardlinks.tar"

	destination, err := ioutil.TempDir("", "archiver_tar_test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(destination)

	err = archiver.DefaultTar.Extract(source, path.Join("dir-1", "dir-2"), destination)
	if err != nil {
		t.Fatalf("unarchiving '%s' to '%s': %v", source, destination, err)
	}

	fileaInfo := requireRegularFile(t, path.Join(destination, "dir-2", "file-a"))
	filebInfo := requireRegularFile(t, path.Join(destination, "dir-2", "file-b"))
	assertSameFile(t, fileaInfo, filebInfo)
}

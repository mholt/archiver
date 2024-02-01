package archiver_test

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/mholt/archiver/v3"
)

func requireDoesNotExist(t *testing.T, path string) {
	_, err := os.Lstat(path)
	if err == nil {
		t.Fatalf("'%s' expected to not exist", path)
	}
}

func requireRegularFile(t *testing.T, path string) os.FileInfo {
	fileInfo, err := os.Lstat(path)
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

func TestDefaultTar_Unarchive_SymlinkPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.tar")
	createSymlinkPathTraversalSample(t, source, "./../target")
	destination := filepath.Join(tmp, "destination")

	err := archiver.DefaultTar.Unarchive(source, destination)
	if err != nil {
		t.Fatalf("unarchiving '%s' to '%s': %v", source, destination, err)
	}

	requireDoesNotExist(t, filepath.Join(tmp, "target"))
	requireRegularFile(t, filepath.Join(tmp, "destination", "duplicatedentry.txt"))
}

func TestDefaultTar_Unarchive_SymlinkPathTraversal_AbsLinkDestination(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.tar")
	createSymlinkPathTraversalSample(t, source, "/tmp/thing")
	destination := filepath.Join(tmp, "destination")

	err := archiver.DefaultTar.Unarchive(source, destination)
	if err != nil {
		t.Fatalf("unarchiving '%s' to '%s': %v", source, destination, err)
	}

	requireDoesNotExist(t, "/tmp/thing")
	requireRegularFile(t, filepath.Join(tmp, "destination", "duplicatedentry.txt"))
}

func createSymlinkPathTraversalSample(t *testing.T, archivePath string, linkPath string) {
	t.Helper()

	type tarinfo struct {
		Name string
		Link string
		Body string
		Type byte
	}

	var infos = []tarinfo{
		{"duplicatedentry.txt", linkPath, "", tar.TypeSymlink},
		{"duplicatedentry.txt", "", "content modified!", tar.TypeReg},
	}

	var buf bytes.Buffer
	var tw = tar.NewWriter(&buf)
	for _, ti := range infos {
		hdr := &tar.Header{
			Name:     ti.Name,
			Mode:     0600,
			Linkname: ti.Link,
			Typeflag: ti.Type,
			Size:     int64(len(ti.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal("Writing header: ", err)
		}
		if _, err := tw.Write([]byte(ti.Body)); err != nil {
			t.Fatal("Writing body: ", err)
		}
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
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

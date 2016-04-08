package archiver

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZip(t *testing.T) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	outfile := filepath.Join(tmp, "test.zip")
	err = Zip(outfile, []string{
		"testdata",
	})
	if err != nil {
		t.Errorf("Didn't expect an error, but got: %v", err)
	}

	var fileCount int
	filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		fileCount++
		return nil
	})

	r, err := zip.OpenReader(outfile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if got, want := len(r.File), fileCount; got != want {
		t.Fatalf("Expected %d files, got %d", want, got)
	}

	for _, zf := range r.File {
		if strings.HasSuffix(zf.Name, "/") {
			// stat dir instead of read file
			_, err := os.Stat(zf.Name)
			if err != nil {
				t.Fatalf("%s: Couldn't stat directory: %v", zf.Name, err)
			}
			continue
		}

		expected, err := ioutil.ReadFile(zf.Name)
		if err != nil {
			t.Fatalf("%s: Couldn't open from disk: %v", zf.Name, err)
		}

		rc, err := zf.Open()
		if err != nil {
			t.Fatalf("%s: Couldn't open compressed file: %v", zf.Name, err)
		}
		actual := new(bytes.Buffer)
		_, err = io.Copy(actual, rc)
		if err != nil {
			t.Fatalf("%s: Couldn't read contents of compressed file: %v", zf.Name, err)
		}
		rc.Close()

		if !bytes.Equal(expected, actual.Bytes()) {
			t.Fatalf("%s: File contents differed between on disk and compressed", zf.Name)
		}
	}
}

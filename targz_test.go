package archiver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestTarGz(t *testing.T) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	outfile := filepath.Join(tmp, "test.tar.gz")
	err = TarGz(outfile, []string{
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

	f, err := os.Open(outfile)
	if err != nil {
		t.Fatalf("%s: Failed to open archive: %v", outfile, err)
	}
	defer f.Close()

	gzf, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to create new gzip reader: %v", err)
	}

	tr := tar.NewReader(gzf)

	i := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Error advancing to next: %v", err)
		}
		i++

		switch header.Typeflag {
		case tar.TypeDir:
			// stat dir instead of read file
			_, err := os.Stat(header.Name)
			if err != nil {
				t.Fatalf("%s: Couldn't stat directory: %v", header.Name, err)
			}
			continue
		case tar.TypeReg:
			expected, err := ioutil.ReadFile(header.Name)
			if err != nil {
				t.Fatalf("%s: Couldn't open from disk: %v", header.Name, err)
			}
			actual := new(bytes.Buffer)
			_, err = io.Copy(actual, tr)
			if err != nil {
				t.Fatalf("%s: Couldn't read contents of compressed file: %v", header.Name, err)
			}
			if !bytes.Equal(expected, actual.Bytes()) {
				t.Fatalf("%s: File contents differed between on disk and compressed", header.Name)
			}
		default:
			t.Fatalf("%s: Unknown type flag: %c", header.Name, header.Typeflag)
		}
	}

	if i != fileCount {
		t.Fatalf("Expected %d files in archive, got %d", fileCount, i)
	}
}

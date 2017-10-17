package archiver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestTar(t *testing.T) {
	abs, e := filepath.Abs("./testdata")
	if e != nil {
		t.Fatal("get absolute path for testdata failed:", e.Error())
	}

	testdataAllForm := []string{
		abs,
		abs + "/",
		"testdata",
		"./testdata",
		".//testdata",
		"./testdata/",
		".//testdata/",
	}

	name := "Tar"
	ar := Tar
	for _, p := range testdataAllForm {
		t.Run(fmt.Sprintf("path=%s", p), func(t *testing.T) {
			t.Parallel()

			tmp, err := ioutil.TempDir("", "archiver")
			if err != nil {
				t.Fatalf("[%s] %v", name, err)
			}
			defer os.RemoveAll(tmp)

			// Test creating archive
			outfile := filepath.Join(tmp, "test-"+name)
			err = ar.Make(outfile, []string{p})
			if err != nil {
				t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
			}

			if !ar.Match(outfile) {
				t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
			}

			// Test extracting archive
			dest := filepath.Join(tmp, "extraction_test")
			os.Mkdir(dest, 0755)
			err = ar.Open(outfile, dest)
			if err != nil {
				t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", name, outfile, dest, err)
			}

			// Check that what was extracted is what was compressed
			symmetricTest(t, name, dest)
		})
	}
}

func TestTarWithDiffDest(t *testing.T) {
	abs, e := filepath.Abs("./testdata")
	if e != nil {
		t.Fatal("get absolute path for testdata failed:", e.Error())
	}

	testdataAllForm := []string{
		abs,
		abs + "/",
		"testdata",
		"./testdata",
		".//testdata",
		"./testdata/",
		".//testdata/",
	}

	name := "Tar"
	ar := Tar
	for _, p := range testdataAllForm {
		t.Run(fmt.Sprintf("path=%s", p), func(t *testing.T) {
			t.Parallel()

			tmp, err := ioutil.TempDir("", "archiver")
			if err != nil {
				t.Fatalf("[%s] %v", name, err)
			}
			defer os.RemoveAll(tmp)

			// Test creating archive
			outfile := filepath.Join(p, "test-"+name)
			err = ar.Make(outfile, []string{p})
			if err != nil {
				t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
			}

			if !ar.Match(outfile) {
				t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
			}

			// Test extracting archive
			dest := filepath.Join(tmp, "extraction_test")
			os.Mkdir(dest, 0755)
			err = ar.Open(outfile, dest)
			if err != nil {
				t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", name, outfile, dest, err)
			}

			testSame(t, name, dest, outfile)
		})
	}
}

// symmetricTest compares the contents of a destination directory to the contents
// of the test corpus and tests that they are equal.
func testSame(t *testing.T, name, dest, ignore string) {
	absForTestdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal("get absolute path for testdata failed:", err.Error())
	}

	absForIgnore, err := filepath.Abs(ignore)
	if err != nil {
		t.Fatal("get absolute path for ignore file failed:", err.Error())
	}

	var expectedFileCount int
	filepath.Walk(absForTestdata, func(fpath string, info os.FileInfo, err error) error {
		if absForIgnore == fpath {
			return nil
		}

		expectedFileCount++
		return nil
	})

	// If outputs equals inputs, we're good; traverse output files
	// and compare file names, file contents, and file count.
	var actualFileCount int
	dest, _ = filepath.Abs(dest)
	filepath.Walk(dest, func(fpath string, info os.FileInfo, err error) error {
		if fpath == dest {
			return nil
		}

		if absForIgnore == fpath {
			return nil
		}

		actualFileCount++

		origPath, err := filepath.Rel(dest, fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error inducing original file path: %v", name, fpath, err)
		}

		if info.IsDir() {
			// stat dir instead of read file
			_, err = os.Stat(origPath)
			if err != nil {
				t.Fatalf("[%s] %s: Couldn't stat original directory (%s): %v", name,
					fpath, origPath, err)
			}
			return nil
		}

		expectedFileInfo, err := os.Stat(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining original file info: %v", name, fpath, err)
		}
		expected, err := ioutil.ReadFile(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open original file (%s) from disk: %v", name,
				fpath, origPath, err)
		}

		actualFileInfo, err := os.Stat(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining actual file info: %v", name, fpath, err)
		}
		actual, err := ioutil.ReadFile(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open new file from disk: %v", name, fpath, err)
		}

		if actualFileInfo.Mode() != expectedFileInfo.Mode() {
			t.Fatalf("[%s] %s: File mode differed between on disk and compressed", name,
				expectedFileInfo.Mode().String()+" : "+actualFileInfo.Mode().String())
		}
		if !bytes.Equal(expected, actual) {
			t.Fatalf("[%s] %s: File contents differed between on disk and compressed", name, origPath)
		}

		return nil
	})

	if got, want := actualFileCount, expectedFileCount; got != want {
		t.Fatalf("[%s] Expected %d resulting files, got %d", name, want, got)
	}
}

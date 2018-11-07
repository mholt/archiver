package archiver

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiver(t *testing.T) {
	for name, ar := range SupportedFormats {
		name, ar := name, ar
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				t.Skip("not supported")
			}

			_, gzOk := ar.(gzFormat)
			_, bzip2Ok := ar.(bzip2Format)
			if gzOk || bzip2Ok {
				testSingleWriteRead(t, name, ar)
				testSingleMakeOpen(t, name, ar)
			} else {
				testWriteRead(t, name, ar)
				testMakeOpen(t, name, ar)
				testMakeOpenWithDestinationEndingInSlash(t, name, ar)
				testMakeOpenNotOverwriteAtDestination(t, name, ar)
			}
		})
	}
}

// testSingleWriteRead performs a symmetric test by using ar.Write to generate
// an archive from the test corpus, then using ar.Read to extract the archive
// and comparing the contents to ensure they are equal.
func testSingleWriteRead(t *testing.T, name string, ar Archiver) {
	buf := new(bytes.Buffer)
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	origPath := "testdata/quote1.txt"
	newPath := filepath.Join(tmp, "quote1.txt")

	// Test creating archive
	err = ar.Write(buf, []string{origPath})
	if err != nil {
		t.Fatalf("[%s] writing archive: didn't expect an error, but got: %v", name, err)
	}

	// Test extracting archive
	err = ar.Read(buf, newPath)
	if err != nil {
		t.Fatalf("[%s] reading archive: didn't expect an error, but got: %v", name, err)
	}

	// Check that what was extracted is what was compressed
	checkSameContent(t, name, origPath, newPath)
}

// testWriteRead performs a symmetric test by using ar.Write to generate an archive
// from the test corpus, then using ar.Read to extract the archive and comparing
// the contents to ensure they are equal.
func testWriteRead(t *testing.T, name string, ar Archiver) {
	buf := new(bytes.Buffer)
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	err = ar.Write(buf, []string{"testdata"})
	if err != nil {
		t.Fatalf("[%s] writing archive: didn't expect an error, but got: %v", name, err)
	}

	// Test extracting archive
	err = ar.Read(buf, tmp)
	if err != nil {
		t.Fatalf("[%s] reading archive: didn't expect an error, but got: %v", name, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, name, tmp)
}

// testSingleMakeOpen performs a symmetric test by using ar.Make to make an archive
// from the test corpus, then using ar.Open to open the archive and comparing
// the contents to ensure they are equal.
func testSingleMakeOpen(t *testing.T, name string, ar Archiver) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	origPath := "testdata/quote1.txt"
	newPath := filepath.Join(tmp, "quote1.txt")

	// Test creating archive
	outfile := filepath.Join(tmp, "test-"+name)
	err = ar.Make(outfile, []string{origPath})
	if err != nil {
		t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
	}

	if !ar.Match(outfile) {
		t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
	}

	// Test extracting archive
	err = ar.Open(outfile, newPath)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", name, outfile, newPath, err)
	}

	// Check that what was extracted is what was compressed
	checkSameContent(t, name, origPath, newPath)
}

// testMakeOpen performs a symmetric test by using ar.Make to make an archive
// from the test corpus, then using ar.Open to open the archive and comparing
// the contents to ensure they are equal.
func testMakeOpen(t *testing.T, name string, ar Archiver) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	outfile := filepath.Join(tmp, "test-"+name)
	err = ar.Make(outfile, []string{"testdata"})
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
}

// testMakeOpenWithDestinationEndingInSlash is similar to testMakeOpen except that
// it tests the case where destination path has a terminating forward slash especially
// on Windows os.
func testMakeOpenWithDestinationEndingInSlash(t *testing.T, name string, ar Archiver) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	outfile := filepath.Join(tmp, "test-"+name)
	err = ar.Make(outfile, []string{"testdata"})
	if err != nil {
		t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
	}

	if !ar.Match(outfile) {
		t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
	}

	// Test extracting archive with destination that has a slash at the end
	dest := filepath.Join(tmp, "extraction_test")
	os.Mkdir(dest, 0755)
	err = ar.Open(outfile, dest+"/")
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", name, outfile, dest, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, name, dest)
}

// testMakeOpenNotOverwriteAtDestination performs a test to ensure we do not overwrite existing files
// when extracting at a destination that has existing files named as those in the archive.
func testMakeOpenNotOverwriteAtDestination(t *testing.T, name string, ar Archiver) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		t.Fatalf("[%s] %v", name, err)
	}
	defer os.RemoveAll(tmp)

	mockOverWriteDirectory := filepath.Join(tmp, "testdatamock")
	mockOverWriteFile := filepath.Join(tmp, "testdatamock", "shouldnotoverwrite.txt")

	// Prepare a mock directory with files for testing
	if err := os.Mkdir(mockOverWriteDirectory, 0774); err != nil {
		t.Fatalf("[%s] preping mock directory: didn't expect an error, but got: %v", mockOverWriteDirectory, err)
	}
	if err := ioutil.WriteFile(filepath.Join(mockOverWriteDirectory, "fileatsourceonly.txt"), []byte("File-From-Source"), 0644); err != nil {
		t.Fatalf("[%s] prep file in source: didn't expect an error, but got: %v", name, err)
	}
	defer os.RemoveAll(mockOverWriteDirectory)
	if err := ioutil.WriteFile(mockOverWriteFile, []byte("File-From-Source"), 0644); err != nil {
		t.Fatalf("[%s] prep file in source: didn't expect an error, but got: %v", name, err)
	}

	// Test creating archive
	outfile := filepath.Join(tmp, "test-"+name)
	err = ar.Make(outfile, []string{mockOverWriteDirectory})
	if err != nil {
		t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", name, err)
	}

	if !ar.Match(outfile) {
		t.Fatalf("[%s] identifying format should be 'true', but got 'false'", name)
	}

	// Introduce a change to the mock file, to track if it would be overwritten by unarchiving.
	if err := ioutil.WriteFile(mockOverWriteFile, []byte("File-At-Destination"), 0644); err != nil {
		t.Fatalf("[%s] change file in destination: didn't expect an error, but got: %v", name, err)
	}

	// Test extracting archive with destination same as original folder
	dest := tmp
	if err := ar.Open(outfile, dest); err != nil {
		if !strings.Contains(err.Error(), "skipping because there exists a file with the same name") {
			t.Fatalf("[%s] extracting archive [%s -> %s]: Unexpected error got: %v", name, outfile, dest, err)
		}
	}

	// Validate if the mock file was changed by the un-archiving process
	content, err := ioutil.ReadFile(mockOverWriteFile)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: Unable to read file : %v", name, outfile, dest, err)
	}
	if string(content) != "File-At-Destination" {
		t.Fatalf("[%s] extracting archive [%s -> %s]: Unexpected Overwrite of File at Destination %s, %s got %s", name, outfile, dest, mockOverWriteFile, "File-At-Destination", string(content))

	}
}

// symmetricTest compares the contents of a destination directory to the contents
// of the test corpus and tests that they are equal.
func symmetricTest(t *testing.T, name, dest string) {
	var expectedFileCount int
	filepath.Walk("testdata", func(fpath string, info os.FileInfo, err error) error {
		expectedFileCount++
		return nil
	})

	// If outputs equals inputs, we're good; traverse output files
	// and compare file names, file contents, and file count.
	var actualFileCount int
	filepath.Walk(dest, func(fpath string, info os.FileInfo, err error) error {
		if fpath == dest {
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
		actualFileInfo, err := os.Stat(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining actual file info: %v", name, fpath, err)
		}
		if actualFileInfo.Mode() != expectedFileInfo.Mode() {
			t.Fatalf("[%s] %s: File mode differed between on disk and compressed", name,
				expectedFileInfo.Mode().String()+" : "+actualFileInfo.Mode().String())
		}

		checkSameContent(t, name, origPath, fpath)

		return nil
	})

	if got, want := actualFileCount, expectedFileCount; got != want {
		t.Fatalf("[%s] Expected %d resulting files, got %d", name, want, got)
	}
}

func checkSameContent(t *testing.T, name, origPath, fpath string) {
	expected, err := ioutil.ReadFile(origPath)
	if err != nil {
		t.Fatalf("[%s] %s: Couldn't open original file (%s) from disk: %v", name,
			fpath, origPath, err)
	}
	actual, err := ioutil.ReadFile(fpath)
	if err != nil {
		t.Fatalf("[%s] %s: Couldn't open new file from disk: %v", name, fpath, err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("[%s] %s: File contents differed between on disk and compressed", name, origPath)
	}
}

func BenchmarkMake(b *testing.B) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for name, ar := range SupportedFormats {
		name, ar := name, ar
		b.Run(name, func(b *testing.B) {
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				b.Skip("not supported")
			}
			outfile := filepath.Join(tmp, "benchMake-"+name)
			for i := 0; i < b.N; i++ {
				err = ar.Make(outfile, []string{"testdata"})
				if err != nil {
					b.Fatalf("making archive: didn't expect an error, but got: %v", err)
				}
			}
		})
	}
}

func BenchmarkOpen(b *testing.B) {
	tmp, err := ioutil.TempDir("", "archiver")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for name, ar := range SupportedFormats {
		name, ar := name, ar
		b.Run(name, func(b *testing.B) {
			// skip RAR for now
			if _, ok := ar.(rarFormat); ok {
				b.Skip("not supported")
			}
			// prepare a archive
			outfile := filepath.Join(tmp, "benchMake-"+name)
			err = ar.Make(outfile, []string{"testdata"})
			if err != nil {
				b.Fatalf("open archive: didn't expect an error, but got: %v", err)
			}
			// prepare extraction destination
			dest := filepath.Join(tmp, "extraction_test")
			os.Mkdir(dest, 0755)

			// let's go
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err = ar.Open(outfile, dest)
				if err != nil {
					b.Fatalf("open archive: didn't expect an error, but got: %v", err)
				}
			}
		})
	}
}

package archiver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TODO: We need a new .rar file since we moved the test corpus into the testdata/corpus subfolder.
/*
func TestRarUnarchive(t *testing.T) {
	au := DefaultRar
	auStr := fmt.Sprintf("%s", au)

	tmp, err := ioutil.TempDir("", "archiver_test")
	if err != nil {
		t.Fatalf("[%s] %v", auStr, err)
	}
	defer os.RemoveAll(tmp)

	dest := filepath.Join(tmp, "extraction_test_"+auStr)
	os.Mkdir(dest, 0755)

	file := "testdata/sample.rar"
	err = au.Unarchive(file, dest)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", auStr, file, dest, err)
	}

	// Check that what was extracted is what was compressed
	// Extracting links isn't implemented yet (in github.com/nwaples/rardecode lib there are no methods to get symlink info)
	// Files access modes may differs on different machines, we are comparing extracted(as archive host) and local git clone
	symmetricTest(t, auStr, dest, false, false)
}
*/

func TestArchiveUnarchive(t *testing.T) {
	for _, af := range archiveFormats {
		au, ok := af.(archiverUnarchiver)
		if !ok {
			t.Errorf("%s (%T): not an Archiver and Unarchiver", af, af)
			continue
		}
		testArchiveUnarchive(t, au)
	}
}

func TestArchiveUnarchiveWithFolderPermissions(t *testing.T) {
	dir := "testdata/corpus/proverbs/extra"
	currentPerms, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = os.Chmod(dir, 0700)
	if err != nil {
		t.Fatalf("%v", err)
	}

	defer func() {
		err := os.Chmod(dir, currentPerms.Mode())
		if err != nil {
			t.Fatalf("%v", err)
		}
	}()

	TestArchiveUnarchive(t)
}

func testArchiveUnarchive(t *testing.T, au archiverUnarchiver) {
	auStr := fmt.Sprintf("%s", au)

	tmp, err := ioutil.TempDir("", "archiver_test")
	if err != nil {
		t.Fatalf("[%s] %v", auStr, err)
	}
	defer os.RemoveAll(tmp)

	// Test creating archive
	outfile := filepath.Join(tmp, "archiver_test."+auStr)
	err = au.Archive([]string{"testdata/corpus"}, outfile)
	if err != nil {
		t.Fatalf("[%s] making archive: didn't expect an error, but got: %v", auStr, err)
	}

	// Test format matching (TODO: Make this its own test, out of band with the archive/unarchive tests)
	//testMatching(t, au, outfile) // TODO: Disabled until we can finish implementing this for compressed tar formats

	// Test extracting archive
	dest := filepath.Join(tmp, "extraction_test_"+auStr)
	_ = os.Mkdir(dest, 0755)
	err = au.Unarchive(outfile, dest)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", auStr, outfile, dest, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, auStr, dest, true, true)
}

/*
// testMatching tests that au can match the format of archiveFile.
func testMatching(t *testing.T, au archiverUnarchiver, archiveFile string) {
	m, ok := au.(Matcher)
	if !ok {
		t.Logf("[NOTICE] %T (%s) is not a Matcher", au, au)
		return
	}

	file, err := os.Open(archiveFile)
	if err != nil {
		t.Fatalf("[%s] opening file for matching: %v", au, err)
	}
	defer file.Close()

	tmpBuf := make([]byte, 2048)
	io.ReadFull(file, tmpBuf)

	matched, err := m.Match(file)
	if err != nil {
		t.Fatalf("%s (%T): testing matching: got error, expected none: %v", m, m, err)
	}
	if !matched {
		t.Fatalf("%s (%T): format should have matched, but didn't", m, m)
	}
}
*/

// symmetricTest compares the contents of a destination directory to the contents
// of the test corpus and tests that they are equal.
func symmetricTest(t *testing.T, formatName, dest string, testSymlinks, testModes bool) {
	var expectedFileCount int
	_ = filepath.Walk("testdata/corpus", func(fpath string, info os.FileInfo, err error) error {
		if testSymlinks || (info.Mode()&os.ModeSymlink) == 0 {
			expectedFileCount++
		}
		return nil
	})

	// If outputs equals inputs, we're good; traverse output files
	// and compare file names, file contents, and file count.
	var actualFileCount int
	_ = filepath.Walk(dest, func(fpath string, info os.FileInfo, _ error) error {
		if fpath == dest {
			return nil
		}
		if testSymlinks || (info.Mode()&os.ModeSymlink) == 0 {
			actualFileCount++
		}

		origPath, err := filepath.Rel(dest, fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error inducing original file path: %v", formatName, fpath, err)
		}
		origPath = filepath.Join("testdata", origPath)

		expectedFileInfo, err := os.Lstat(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining original file info: %v", formatName, fpath, err)
		}
		if !testSymlinks && (expectedFileInfo.Mode()&os.ModeSymlink) != 0 {
			return nil
		}
		actualFileInfo, err := os.Lstat(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Error obtaining actual file info: %v", formatName, fpath, err)
		}

		if testModes && actualFileInfo.Mode() != expectedFileInfo.Mode() {
			t.Fatalf("[%s] %s: File mode differed between on disk and compressed", formatName,
				expectedFileInfo.Mode().String()+" : "+actualFileInfo.Mode().String())
		}

		if info.IsDir() {
			// stat dir instead of read file
			_, err = os.Stat(origPath)
			if err != nil {
				t.Fatalf("[%s] %s: Couldn't stat original directory (%s): %v", formatName,
					fpath, origPath, err)
			}
			return nil
		}

		if (actualFileInfo.Mode() & os.ModeSymlink) != 0 {
			expectedLinkTarget, err := os.Readlink(origPath)
			if err != nil {
				t.Fatalf("[%s] %s: Couldn't read original symlink target: %v", formatName, origPath, err)
			}
			actualLinkTarget, err := os.Readlink(fpath)
			if err != nil {
				t.Fatalf("[%s] %s: Couldn't read actual symlink target: %v", formatName, fpath, err)
			}
			if expectedLinkTarget != actualLinkTarget {
				t.Fatalf("[%s] %s: Symlink targets differed between on disk and compressed", formatName, origPath)
			}
			return nil
		}

		expected, err := ioutil.ReadFile(origPath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open original file (%s) from disk: %v", formatName,
				fpath, origPath, err)
		}
		actual, err := ioutil.ReadFile(fpath)
		if err != nil {
			t.Fatalf("[%s] %s: Couldn't open new file from disk: %v", formatName, fpath, err)
		}

		if !bytes.Equal(expected, actual) {
			t.Fatalf("[%s] %s: File contents differed between on disk and compressed", formatName, origPath)
		}

		return nil
	})

	if got, want := actualFileCount, expectedFileCount; got != want {
		t.Fatalf("[%s] Expected %d resulting files, got %d", formatName, want, got)
	}
}

func TestUnarchiveWithStripComponents(t *testing.T) {
	testArchives := []string{
		"testdata/sample.rar",
		"testdata/testarchives/evilarchives/evil.zip",
		"testdata/testarchives/evilarchives/evil.tar",
		"testdata/testarchives/evilarchives/evil.tar.gz",
		"testdata/testarchives/evilarchives/evil.tar.bz2",
	}

	to := "testdata/testarchives/destarchives/"

	for _, archiveName := range testArchives {
		f, err := ByExtension(archiveName)

		if err != nil {
			t.Error(err)
		}

		var target string

		switch v := f.(type) {
		case *Rar:
			v.OverwriteExisting = false
			v.ImplicitTopLevelFolder = false
			v.StripComponents = 1
			target = "quote1.txt"
		case *Zip:
		case *Tar:
			v.OverwriteExisting = false
			v.ImplicitTopLevelFolder = false
			v.StripComponents = 1
			target = "safefile"
		case *TarGz:
		case *TarBz2:
			v.Tar.OverwriteExisting = false
			v.Tar.ImplicitTopLevelFolder = false
			v.Tar.StripComponents = 1
			target = "safefile"
		}

		u := f.(Unarchiver)

		if err := u.Unarchive(archiveName, to); err != nil {
			fmt.Println(err)
		}

		if _, err := os.Stat(filepath.Join(to, target)); os.IsNotExist(err) {
			t.Errorf("file is incorrectly extracted: %s", target)
		}

		os.RemoveAll(to)
	}
}

// test at runtime if the CheckFilename function is behaving properly for the archive formats
func TestSafeExtraction(t *testing.T) {

	testArchives := []string{
		"testdata/testarchives/evilarchives/evil.zip",
		"testdata/testarchives/evilarchives/evil.tar",
		"testdata/testarchives/evilarchives/evil.tar.gz",
		"testdata/testarchives/evilarchives/evil.tar.bz2",
	}

	for _, archiveName := range testArchives {

		expected := true // 'evilfile' should not be extracted outside of destination directory and 'safefile' should be extracted anyway in the destination folder anyway

		if _, err := os.Stat(archiveName); os.IsNotExist(err) {
			t.Errorf("archive not found")
		}

		actual := CheckFilenames(archiveName)

		if actual != expected {
			t.Errorf("CheckFilename is misbehaving for archive format type %s", filepath.Ext(archiveName))
		}
	}
}

func CheckFilenames(archiveName string) bool {

	evilNotExtracted := false // by default we cannot assume that the path traversal filename is mitigated by CheckFilename
	safeExtracted := false    // by default we cannot assume that a benign file can be extracted successfully

	// clean the destination folder after this test
	defer os.RemoveAll("testdata/testarchives/destarchives/")

	err := Unarchive(archiveName, "testdata/testarchives/destarchives/")
	if err != nil {
		fmt.Println(err)
	}

	// is 'evilfile' prevented to be extracted outside of the destination folder?
	if _, err := os.Stat("testdata/testarchives/evilfile"); os.IsNotExist(err) {
		evilNotExtracted = true
	}
	// is 'safefile' safely extracted without errors inside the destination path?
	if _, err := os.Stat("testdata/testarchives/destarchives/safedir/safefile"); !os.IsNotExist(err) {
		safeExtracted = true
	}

	return evilNotExtracted && safeExtracted
}

var archiveFormats = []interface{}{
	DefaultZip,
	DefaultTar,
	DefaultTarBrotli,
	DefaultTarBz2,
	DefaultTarGz,
	DefaultTarLz4,
	DefaultTarSz,
	DefaultTarXz,
	DefaultTarZstd,
}

type archiverUnarchiver interface {
	Archiver
	Unarchiver
}

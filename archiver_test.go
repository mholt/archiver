package archiver

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithin(t *testing.T) {
	for i, tc := range []struct {
		path1, path2 string
		expect       bool
	}{
		{
			path1:  "/foo",
			path2:  "/foo/bar",
			expect: true,
		},
		{
			path1:  "/foo",
			path2:  "/foobar/asdf",
			expect: false,
		},
		{
			path1:  "/foobar/",
			path2:  "/foobar/asdf",
			expect: true,
		},
		{
			path1:  "/foobar/asdf",
			path2:  "/foobar",
			expect: false,
		},
		{
			path1:  "/foobar/asdf",
			path2:  "/foobar/",
			expect: false,
		},
		{
			path1:  "/",
			path2:  "/asdf",
			expect: true,
		},
		{
			path1:  "/asdf",
			path2:  "/asdf",
			expect: true,
		},
		{
			path1:  "/",
			path2:  "/",
			expect: true,
		},
		{
			path1:  "/foo/bar/daa",
			path2:  "/foo",
			expect: false,
		},
		{
			path1:  "/foo/",
			path2:  "/foo/bar/daa",
			expect: true,
		},
	} {
		actual := within(tc.path1, tc.path2)
		if actual != tc.expect {
			t.Errorf("Test %d: [%s %s] Expected %t but got %t", i, tc.path1, tc.path2, tc.expect, actual)
		}
	}
}

func TestMultipleTopLevels(t *testing.T) {
	for i, tc := range []struct {
		set    []string
		expect bool
	}{
		{
			set:    []string{},
			expect: false,
		},
		{
			set:    []string{"/foo"},
			expect: false,
		},
		{
			set:    []string{"/foo", "/foo/bar"},
			expect: false,
		},
		{
			set:    []string{"/foo", "/bar"},
			expect: true,
		},
		{
			set:    []string{"/foo", "/foobar"},
			expect: true,
		},
		{
			set:    []string{"foo", "foo/bar"},
			expect: false,
		},
		{
			set:    []string{"foo", "/foo/bar"},
			expect: false,
		},
		{
			set:    []string{"../foo", "foo/bar"},
			expect: true,
		},
		{
			set:    []string{`C:\foo\bar`, `C:\foo\bar\zee`},
			expect: false,
		},
		{
			set:    []string{`C:\`, `C:\foo\bar`},
			expect: false,
		},
		{
			set:    []string{`D:\foo`, `E:\foo`},
			expect: true,
		},
		{
			set:    []string{`D:\foo`, `D:\foo\bar`, `C:\foo`},
			expect: true,
		},
		{
			set:    []string{"/foo", "/", "/bar"},
			expect: true,
		},
	} {
		actual := multipleTopLevels(tc.set)
		if actual != tc.expect {
			t.Errorf("Test %d: %v: Expected %t but got %t", i, tc.set, tc.expect, actual)
		}
	}
}

func TestMakeNameInArchive(t *testing.T) {
	for i, tc := range []struct {
		sourceInfo fakeFileInfo
		source     string // a file path explicitly listed by the user to include in the archive
		baseDir    string // the base or root directory or path within the archive which contains all other files
		fpath      string // the file path being walked; if source is a directory, this will be a child path
		expect     string
	}{
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "foo.txt",
			baseDir:    "",
			fpath:      "foo.txt",
			expect:     "foo.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "foo.txt",
			baseDir:    "base",
			fpath:      "foo.txt",
			expect:     "base/foo.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "foo/bar.txt",
			baseDir:    "",
			fpath:      "foo/bar.txt",
			expect:     "bar.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "foo/bar.txt",
			baseDir:    "base",
			fpath:      "foo/bar.txt",
			expect:     "base/bar.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: true},
			source:     "foo/bar",
			baseDir:    "base",
			fpath:      "foo/bar",
			expect:     "base/bar",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "/absolute/path.txt",
			baseDir:    "",
			fpath:      "/absolute/path.txt",
			expect:     "path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "/absolute/sub/path.txt",
			baseDir:    "",
			fpath:      "/absolute/sub/path.txt",
			expect:     "path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "/absolute/sub/path.txt",
			baseDir:    "base",
			fpath:      "/absolute/sub/path.txt",
			expect:     "base/path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: false},
			source:     "sub/path.txt",
			baseDir:    "base/subbase",
			fpath:      "sub/path.txt",
			expect:     "base/subbase/path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: true},
			source:     "sub/dir",
			baseDir:    "base/subbase",
			fpath:      "sub/dir/path.txt",
			expect:     "base/subbase/dir/path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: true},
			source:     "sub/dir",
			baseDir:    "base/subbase",
			fpath:      "sub/dir/sub2/sub3/path.txt",
			expect:     "base/subbase/dir/sub2/sub3/path.txt",
		},
		{
			sourceInfo: fakeFileInfo{isDir: true},
			source:     `/absolute/dir`,
			baseDir:    "base",
			fpath:      `/absolute/dir/sub1/sub2/file.txt`,
			expect:     "base/dir/sub1/sub2/file.txt",
		},
	} {
		actual, err := makeNameInArchive(tc.sourceInfo, tc.source, tc.baseDir, tc.fpath)
		if err != nil {
			t.Errorf("Test %d: Got error: %v", i, err)
		}
		if actual != tc.expect {
			t.Errorf("Test %d: Expected '%s' but got '%s'", i, tc.expect, actual)
		}
	}
}

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
	os.Mkdir(dest, 0755)
	err = au.Unarchive(outfile, dest)
	if err != nil {
		t.Fatalf("[%s] extracting archive [%s -> %s]: didn't expect an error, but got: %v", auStr, outfile, dest, err)
	}

	// Check that what was extracted is what was compressed
	symmetricTest(t, auStr, dest, true, true)
}

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

// symmetricTest compares the contents of a destination directory to the contents
// of the test corpus and tests that they are equal.
func symmetricTest(t *testing.T, formatName, dest string, testSymlinks, testModes bool) {
	var expectedFileCount int
	filepath.Walk("testdata/corpus", func(fpath string, info os.FileInfo, err error) error {
		if testSymlinks || (info.Mode()&os.ModeSymlink) == 0 {
			expectedFileCount++
		}
		return nil
	})

	// If outputs equals inputs, we're good; traverse output files
	// and compare file names, file contents, and file count.
	var actualFileCount int
	filepath.Walk(dest, func(fpath string, info os.FileInfo, err error) error {
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

type fakeFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (ffi fakeFileInfo) Name() string       { return ffi.name }
func (ffi fakeFileInfo) Size() int64        { return ffi.size }
func (ffi fakeFileInfo) Mode() os.FileMode  { return ffi.mode }
func (ffi fakeFileInfo) ModTime() time.Time { return ffi.modTime }
func (ffi fakeFileInfo) IsDir() bool        { return ffi.isDir }
func (ffi fakeFileInfo) Sys() interface{}   { return ffi.sys }

package archiver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExtension(t *testing.T) {
	testdir, err := ioutil.TempDir("", "archiver_checkext_test_")
	if err != nil {
		t.Fatalf("Making temporary directory: %v", err)
	}
	defer os.RemoveAll(testdir)
	testfile, err := ioutil.TempFile(testdir, "compressor_test_input_*.txt")
	if err != nil {
		t.Fatalf("Making temporary file: %v", err)
	}
	defer os.Remove(testfile.Name())
	defer testfile.Close()

	for i, tc := range []struct {
		checker   ExtensionChecker
		ext       string // including leading dot
		shouldErr bool
	}{
		{checker: NewBz2(), ext: ".bz2", shouldErr: false},
		{checker: NewBz2(), ext: ".gz", shouldErr: true},

		{checker: NewGz(), ext: ".gz", shouldErr: false},
		{checker: NewGz(), ext: ".sz", shouldErr: true},

		{checker: NewLz4(), ext: ".lz4", shouldErr: false},
		{checker: NewLz4(), ext: ".xz", shouldErr: true},

		{checker: NewSnappy(), ext: ".sz", shouldErr: false},
		{checker: NewSnappy(), ext: ".lz4", shouldErr: true},

		{checker: NewXz(), ext: ".xz", shouldErr: false},
		{checker: NewXz(), ext: ".bz2", shouldErr: true},

		{checker: NewZip(), ext: ".zip", shouldErr: false},
		{checker: NewZip(), ext: ".zip.gz", shouldErr: true},
		{checker: NewZip(), ext: ".tgz", shouldErr: true},
		{checker: NewZip(), ext: ".gz", shouldErr: true},

		{checker: NewTar(), ext: ".tar", shouldErr: false},
		{checker: NewTar(), ext: ".zip", shouldErr: true},
		{checker: NewTar(), ext: ".tar.gz", shouldErr: true},
		{checker: NewTar(), ext: ".tgz", shouldErr: true},

		{checker: NewTarBz2(), ext: ".tar.bz2", shouldErr: false},
		{checker: NewTarBz2(), ext: ".tbz2", shouldErr: false},
		{checker: NewTarBz2(), ext: ".zip", shouldErr: true},
		{checker: NewTarBz2(), ext: ".tar", shouldErr: true},
		{checker: NewTarBz2(), ext: ".bz2", shouldErr: true},

		{checker: NewTarGz(), ext: ".tar.gz", shouldErr: false},
		{checker: NewTarGz(), ext: ".tgz", shouldErr: false},
		{checker: NewTarGz(), ext: ".zip", shouldErr: true},
		{checker: NewTarGz(), ext: ".tar", shouldErr: true},
		{checker: NewTarGz(), ext: ".gz", shouldErr: true},

		{checker: NewTarLz4(), ext: ".tar.lz4", shouldErr: false},
		{checker: NewTarLz4(), ext: ".tlz4", shouldErr: false},
		{checker: NewTarLz4(), ext: ".zip", shouldErr: true},
		{checker: NewTarLz4(), ext: ".tar", shouldErr: true},
		{checker: NewTarLz4(), ext: ".lz4", shouldErr: true},

		{checker: NewTarSz(), ext: ".tar.sz", shouldErr: false},
		{checker: NewTarSz(), ext: ".tsz", shouldErr: false},
		{checker: NewTarSz(), ext: ".zip", shouldErr: true},
		{checker: NewTarSz(), ext: ".tar", shouldErr: true},
		{checker: NewTarSz(), ext: ".sz", shouldErr: true},

		{checker: NewTarXz(), ext: ".tar.xz", shouldErr: false},
		{checker: NewTarXz(), ext: ".txz", shouldErr: false},
		{checker: NewTarXz(), ext: ".zip", shouldErr: true},
		{checker: NewTarXz(), ext: ".tar", shouldErr: true},
		{checker: NewTarXz(), ext: ".xz", shouldErr: true},
	} {
		err := tc.checker.CheckExt("test" + tc.ext)
		if tc.shouldErr && err == nil {
			t.Errorf("Test %d [%s - %s]: Expected an error when checking extension, but got none",
				i, tc.checker, tc.ext)
		}
		if !tc.shouldErr && err != nil {
			t.Errorf("Test %d [%s - %s]: Did not expect an error when checking extension, but got: %v",
				i, tc.checker, tc.ext, err)
		}

		// also ensure that methods which create files check the extension,
		// to avoid confusion where the extension indicates one format but
		// actual format is another
		if a, ok := tc.checker.(Archiver); ok {
			filename := fmt.Sprintf("test%d_archive%s", i, tc.ext)
			err := a.Archive(nil, filepath.Join(testdir, filename))
			if tc.shouldErr && err == nil {
				t.Errorf("Test %d [%s - %s]: Archive(): Expected an error with filename '%s' but got none",
					i, tc.checker, tc.ext, filename)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Test %d [%s - %s]: Archive(): Did not expect an error with filename '%s', but got: %v",
					i, tc.checker, tc.ext, filename, err)
			}
		}
		if c, ok := tc.checker.(FileCompressor); ok {
			filename := fmt.Sprintf("test%d_compress%s", i, tc.ext)
			err := c.CompressFile(testfile.Name(), filepath.Join(testdir, filename))
			if tc.shouldErr && err == nil {
				t.Errorf("Test %d [%s - %s]: Compress(): Expected an error with filename '%s' but got none",
					i, tc.checker, tc.ext, filename)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Test %d [%s - %s]: Compress(): Did not expect an error with filename '%s', but got: %v",
					i, tc.checker, tc.ext, filename, err)
			}
		}
	}
}

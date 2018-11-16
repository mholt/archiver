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

	fmt.Println("TEST DIR:", testdir)
	fmt.Println("TEST FILE:", testfile.Name())

	for i, tc := range []struct {
		checker   ExtensionChecker
		ext       string // including leading dot
		shouldErr bool
	}{
		{checker: &Bz2{}, ext: ".bz2", shouldErr: false},
		{checker: &Bz2{}, ext: ".gz", shouldErr: true},

		{checker: &Gz{}, ext: ".gz", shouldErr: false},
		{checker: &Gz{}, ext: ".sz", shouldErr: true},

		{checker: &Lz4{}, ext: ".lz4", shouldErr: false},
		{checker: &Lz4{}, ext: ".xz", shouldErr: true},

		{checker: &Snappy{}, ext: ".sz", shouldErr: false},
		{checker: &Snappy{}, ext: ".lz4", shouldErr: true},

		{checker: &Xz{}, ext: ".xz", shouldErr: false},
		{checker: &Xz{}, ext: ".bz2", shouldErr: true},

		{checker: DefaultZip, ext: ".zip", shouldErr: false},
		{checker: DefaultZip, ext: ".zip.gz", shouldErr: true},
		{checker: DefaultZip, ext: ".tgz", shouldErr: true},
		{checker: DefaultZip, ext: ".gz", shouldErr: true},

		{checker: DefaultTar, ext: ".tar", shouldErr: false},
		{checker: DefaultTar, ext: ".zip", shouldErr: true},
		{checker: DefaultTar, ext: ".tar.gz", shouldErr: true},
		{checker: DefaultTar, ext: ".tgz", shouldErr: true},

		{checker: DefaultTarBz2, ext: ".tar.bz2", shouldErr: false},
		{checker: DefaultTarBz2, ext: ".tbz2", shouldErr: false},
		{checker: DefaultTarBz2, ext: ".zip", shouldErr: true},
		{checker: DefaultTarBz2, ext: ".tar", shouldErr: true},
		{checker: DefaultTarBz2, ext: ".bz2", shouldErr: true},

		{checker: DefaultTarGz, ext: ".tar.gz", shouldErr: false},
		{checker: DefaultTarGz, ext: ".tgz", shouldErr: false},
		{checker: DefaultTarGz, ext: ".zip", shouldErr: true},
		{checker: DefaultTarGz, ext: ".tar", shouldErr: true},
		{checker: DefaultTarGz, ext: ".gz", shouldErr: true},

		{checker: DefaultTarLz4, ext: ".tar.lz4", shouldErr: false},
		{checker: DefaultTarLz4, ext: ".tlz4", shouldErr: false},
		{checker: DefaultTarLz4, ext: ".zip", shouldErr: true},
		{checker: DefaultTarLz4, ext: ".tar", shouldErr: true},
		{checker: DefaultTarLz4, ext: ".lz4", shouldErr: true},

		{checker: DefaultTarSz, ext: ".tar.sz", shouldErr: false},
		{checker: DefaultTarSz, ext: ".tsz", shouldErr: false},
		{checker: DefaultTarSz, ext: ".zip", shouldErr: true},
		{checker: DefaultTarSz, ext: ".tar", shouldErr: true},
		{checker: DefaultTarSz, ext: ".sz", shouldErr: true},

		{checker: DefaultTarXz, ext: ".tar.xz", shouldErr: false},
		{checker: DefaultTarXz, ext: ".txz", shouldErr: false},
		{checker: DefaultTarXz, ext: ".zip", shouldErr: true},
		{checker: DefaultTarXz, ext: ".tar", shouldErr: true},
		{checker: DefaultTarXz, ext: ".xz", shouldErr: true},
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

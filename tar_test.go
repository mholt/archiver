package archiver

import (
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

			// os.RemoveAll("./result")
			// os.Mkdir("./result", 0755)
			// // defer os.Remove("./result")

			// dst := "./result/test.tar"
			// err = Tar.Make(dst, []string{".//testdata"})
			// if err != nil {
			// 	t.Fatal("make tar failed:", err.Error())
			// }

			// extraction := "./result/extraction"
			// err = Tar.Open(dst, extraction)
			// if err != nil {
			// 	t.Fatal("extract tar failed:", err.Error())
			// }

			// symmetricTest(t, "Tar", extraction)
		})
	}
}

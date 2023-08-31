package archiver

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"reflect"
	"sort"
	"testing"
)

func TestPathWithoutTopDir(t *testing.T) {
	for i, tc := range []struct {
		input, expect string
	}{
		{
			input:  "a/b/c",
			expect: "b/c",
		},
		{
			input:  "b/c",
			expect: "c",
		},
		{
			input:  "c",
			expect: "c",
		},
		{
			input:  "",
			expect: "",
		},
	} {
		if actual := pathWithoutTopDir(tc.input); actual != tc.expect {
			t.Errorf("Test %d (input=%s): Expected '%s' but got '%s'", i, tc.input, tc.expect, actual)
		}
	}
}

//go:generate zip testdata/test.zip go.mod
//go:generate zip -qr9 testdata/nodir.zip archiver.go go.mod cmd/arc/main.go .github/ISSUE_TEMPLATE/bug_report.md .github/FUNDING.yml README.md .github/workflows/ubuntu-latest.yml

var (
	//go:embed testdata/test.zip
	testZIP []byte
	//go:embed testdata/nodir.zip
	nodirZIP []byte
	//go:embed testdata/unordered.zip
	unorderZip []byte
)

func TestSelfTar(t *testing.T) {
	fn := "testdata/self-tar.tar"
	fh, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Could not load test tar: %v", fn)
	}
	fstat, err := os.Stat(fn)
	if err != nil {
		t.Fatalf("Could not stat test tar: %v", fn)
	}
	fsys := ArchiveFS{
		Stream: io.NewSectionReader(fh, 0, fstat.Size()),
		Format: Tar{},
	}
	var count int
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if count > 10 {
			t.Error("walking test tar appears to be recursing in error")
			return fmt.Errorf("recursing tar: %v", fn)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func ExampleArchiveFS_Stream() {
	fsys := ArchiveFS{
		Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
		Format: Zip{},
	}
	// You can serve the contents in a web server:
	http.Handle("/static", http.StripPrefix("/static",
		http.FileServer(http.FS(fsys))))

	// Or read the files using fs functions:
	dis, err := fsys.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, di := range dis {
		fmt.Println(di.Name())
		b, err := fs.ReadFile(fsys, path.Join(".", di.Name()))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(bytes.Contains(b, []byte("require (")))
	}
	// Output:
	// go.mod
	// true
}

func TestArchiveFS_ReadDir(t *testing.T) {
	for _, tc := range []struct {
		name    string
		archive ArchiveFS
		want    map[string][]string
	}{
		{
			name: "test.zip",
			archive: ArchiveFS{
				Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
				Format: Zip{},
			},
			// unzip -l testdata/test.zip
			want: map[string][]string{
				".": {"go.mod"},
			},
		},
		{
			name: "nodir.zip",
			archive: ArchiveFS{
				Stream: io.NewSectionReader(bytes.NewReader(nodirZIP), 0, int64(len(nodirZIP))),
				Format: Zip{},
			},
			// unzip -l testdata/nodir.zip
			want: map[string][]string{
				".":       {".github", "README.md", "archiver.go", "cmd", "go.mod"},
				".github": {"FUNDING.yml", "ISSUE_TEMPLATE", "workflows"},
				"cmd":     {"arc"},
			},
		},
		{
			name: "unordered.zip",
			archive: ArchiveFS{
				Stream: io.NewSectionReader(bytes.NewReader(unorderZip), 0, int64(len(unorderZip))),
				Format: Zip{},
			},
			// unzip -l testdata/unordered.zip, note entry 1/1 and 1/2 are separated by contents of directory 2
			want: map[string][]string{
				".": {"1", "2"},
				"1": {"1", "2"},
				"2": {"1"},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fsys := tc.archive
			for baseDir, wantLS := range tc.want {
				baseDir := baseDir
				wantLS := wantLS
				t.Run(fmt.Sprintf("ReadDir(%s)", baseDir), func(t *testing.T) {
					dis, err := fsys.ReadDir(baseDir)
					if err != nil {
						t.Error(err)
					}

					dirs := []string{}
					for _, di := range dis {
						dirs = append(dirs, di.Name())
					}

					// Stabilize the sort order
					sort.Strings(dirs)

					if !reflect.DeepEqual(wantLS, dirs) {
						t.Errorf("ReadDir() got: %v, want: %v", dirs, wantLS)
					}
				})

				// Uncomment to reproduce https://github.com/mholt/archiver/issues/340.
				t.Run(fmt.Sprintf("Open(%s)", baseDir), func(t *testing.T) {
					f, err := fsys.Open(baseDir)
					if err != nil {
						t.Error(err)
					}

					rdf, ok := f.(fs.ReadDirFile)
					if !ok {
						t.Fatalf("'%s' did not return a fs.ReadDirFile, %+v", baseDir, rdf)
					}

					dis, err := rdf.ReadDir(-1)
					if err != nil {
						t.Fatal(err)
					}

					dirs := []string{}
					for _, di := range dis {
						dirs = append(dirs, di.Name())
					}

					// Stabilize the sort order
					sort.Strings(dirs)

					if !reflect.DeepEqual(wantLS, dirs) {
						t.Errorf("Open().ReadDir(-1) got: %v, want: %v", dirs, wantLS)
					}
				})
			}
		})
	}
}

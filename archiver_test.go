package archiver

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestTrimTopDir(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "b/c"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "def"},
		{input: "/abc/def", want: "def"},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := trimTopDir(tc.input)
			if got != tc.want {
				t.Errorf("want: '%s', got: '%s')", tc.want, got)
			}
		})
	}
}

func TestTopDir(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "a"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "abc"},
		{input: "/abc/def", want: "abc"},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := topDir(tc.input)
			if got != tc.want {
				t.Errorf("want: '%s', got: '%s')", tc.want, got)
			}
		})
	}
}

func TestFileIsIncluded(t *testing.T) {
	for i, tc := range []struct {
		included  []string
		candidate string
		expect    bool
	}{
		{
			included:  []string{"a"},
			candidate: "a",
			expect:    true,
		},
		{
			included:  []string{"a", "b", "a/b"},
			candidate: "b",
			expect:    true,
		},
		{
			included:  []string{"a", "b", "c/d"},
			candidate: "c/d/e",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "a/b/c",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "aa/b/c",
			expect:    false,
		},
		{
			included:  []string{"a", "b", "c/d"},
			candidate: "b/c",
			expect:    true,
		},
		{
			included:  []string{"a/"},
			candidate: "a",
			expect:    false,
		},
		{
			included:  []string{"a/"},
			candidate: "a/",
			expect:    true,
		},
		{
			included:  []string{"a"},
			candidate: "a/",
			expect:    true,
		},
		{
			included:  []string{"a/b"},
			candidate: "a/",
			expect:    false,
		},
	} {
		actual := fileIsIncluded(tc.included, tc.candidate)
		if actual != tc.expect {
			t.Errorf("Test %d (included=%v candidate=%v): expected %t but got %t",
				i, tc.included, tc.candidate, tc.expect, actual)
		}
	}
}

func TestSkipList(t *testing.T) {
	for i, tc := range []struct {
		start  skipList
		add    string
		expect skipList
	}{
		{
			start:  skipList{"a", "b", "c"},
			add:    "d",
			expect: skipList{"a", "b", "c", "d"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b",
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c", // don't add because b implies b/c
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c/", // effectively same as above
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b/", "c"},
			add:    "b", // effectively same as b/
			expect: skipList{"a", "b/", "c"},
		},
		{
			start:  skipList{"a", "b/c", "c"},
			add:    "b", // replace b/c because b is broader
			expect: skipList{"a", "c", "b"},
		},
	} {
		start := make(skipList, len(tc.start))
		copy(start, tc.start)

		tc.start.add(tc.add)

		if !reflect.DeepEqual(tc.start, tc.expect) {
			t.Errorf("Test %d (start=%v add=%v): expected %v but got %v",
				i, start, tc.add, tc.expect, tc.start)
		}
	}
}

func TestNameOnDiskToNameInArchive(t *testing.T) {
	for i, tc := range []struct {
		windows       bool   // only run this test on Windows
		rootOnDisk    string // user says they want to archive this file/folder
		nameOnDisk    string // the walk encounters a file with this name (with rootOnDisk as a prefix)
		rootInArchive string // file should be placed in this dir within the archive (rootInArchive becomes a prefix)
		expect        string // final filename in archive
	}{
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "a/b/c",
		},
		{
			rootOnDisk:    "a/b",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "b/c",
		},
		{
			rootOnDisk:    "a/b/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b/",
			nameOnDisk:    "a/b/c",
			rootInArchive: ".",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b/c",
			nameOnDisk:    "a/b/c",
			rootInArchive: "",
			expect:        "c",
		},
		{
			rootOnDisk:    "a/b",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/c",
		},
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			rootOnDisk:    "a",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo/",
			expect:        "foo/a/b/c",
		},
		{
			rootOnDisk:    "a/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			rootOnDisk:    "a/",
			nameOnDisk:    "a/b/c",
			rootInArchive: "foo",
			expect:        "foo/b/c",
		},
		{
			windows:       true,
			rootOnDisk:    `C:\foo`,
			nameOnDisk:    `C:\foo\bar`,
			rootInArchive: "",
			expect:        "foo/bar",
		},
		{
			windows:       true,
			rootOnDisk:    `C:\foo`,
			nameOnDisk:    `C:\foo\bar`,
			rootInArchive: "subfolder",
			expect:        "subfolder/bar",
		},
	} {
		if !strings.HasPrefix(tc.nameOnDisk, tc.rootOnDisk) {
			t.Fatalf("Test %d: Invalid test case! Filename (on disk) will have rootOnDisk as a prefix according to the fs.WalkDirFunc godoc.", i)
		}
		if tc.windows && runtime.GOOS != "windows" {
			t.Logf("Test %d: Skipping test that is only compatible with Windows", i)
			continue
		}
		if !tc.windows && runtime.GOOS == "windows" {
			t.Logf("Test %d: Skipping test that is not compatible with Windows", i)
			continue
		}

		actual := nameOnDiskToNameInArchive(tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
		if actual != tc.expect {
			t.Errorf("Test %d: Got '%s' but expected '%s' (nameOnDisk=%s rootOnDisk=%s rootInArchive=%s)",
				i, actual, tc.expect, tc.nameOnDisk, tc.rootOnDisk, tc.rootInArchive)
		}
	}
}

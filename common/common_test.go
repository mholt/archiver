package common

import (
	"os"
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
		actual := Within(tc.path1, tc.path2)
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
		actual := MultipleTopLevels(tc.set)
		if actual != tc.expect {
			t.Errorf("Test %d: %v: Expected %t but got %t", i, tc.set, tc.expect, actual)
		}
	}
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
		actual, err := MakeNameInArchive(tc.sourceInfo, tc.source, tc.baseDir, tc.fpath)
		if err != nil {
			t.Errorf("Test %d: Got error: %v", i, err)
		}
		if actual != tc.expect {
			t.Errorf("Test %d: Expected '%s' but got '%s'", i, tc.expect, actual)
		}
	}
}

package archive

import "testing"

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

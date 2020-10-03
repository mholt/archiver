package archiver_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/mholt/archiver/v3"
)

func TestIllegalPathErrorString(t *testing.T) {
	tests := []struct {
		instance *archiver.IllegalPathError
		expected string
	}{
		{instance: &archiver.IllegalPathError{Filename: "foo.txt"}, expected: "illegal file path: foo.txt"},
		{instance: &archiver.IllegalPathError{AbsolutePath: "/tmp/bar.txt", Filename: "bar.txt"}, expected: "illegal file path: bar.txt"},
	}

	for i, test := range tests {
		test := test

		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			if test.expected != test.instance.Error() {
				t.Fatalf("Excepected '%s', but got '%s'", test.expected, test.instance.Error())
			}
		})
	}
}

func TestIsIllegalPathError(t *testing.T) {
	tests := []struct {
		instance error
		expected bool
	}{
		{instance: nil, expected: false},
		{instance: os.ErrNotExist, expected: false},
		{instance: fmt.Errorf("some error"), expected: false},
		{instance: errors.New("another error"), expected: false},
		{instance: &archiver.IllegalPathError{Filename: "foo.txt"}, expected: true},
	}

	for i, test := range tests {
		test := test

		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			actual := archiver.IsIllegalPathError(test.instance)
			if actual != test.expected {
				t.Fatalf("Excepected '%v', but got '%v'", test.expected, actual)
			}
		})
	}
}

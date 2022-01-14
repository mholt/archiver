package archiver

import (
	"reflect"
	"testing"
)

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

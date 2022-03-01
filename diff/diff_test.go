package diff_test

import (
	"testing"

	"github.com/brunokim/crdt/diff"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   []diff.Operation
	}{
		{
			s1: "a",
			s2: "a",
			want: []diff.Operation{
				{Op: diff.Keep, Char: 'a'},
			},
		},
		{
			s1: "",
			s2: "a",
			want: []diff.Operation{
				{Op: diff.Insert, Char: 'a'},
			},
		},
		{
			s1: "a",
			s2: "",
			want: []diff.Operation{
				{Op: diff.Delete, Char: 'a'},
			},
		},
		{
			s1: "abc",
			s2: "abc",
			want: []diff.Operation{
				{Op: diff.Keep, Char: 'a'},
				{Op: diff.Keep, Char: 'b'},
				{Op: diff.Keep, Char: 'c'},
			},
		},
		{
			s1: "ac",
			s2: "abc",
			want: []diff.Operation{
				{Op: diff.Keep, Char: 'a'},
				{Op: diff.Insert, Char: 'b'},
				{Op: diff.Keep, Char: 'c'},
			},
		},
		{
			s1: "abc",
			s2: "ac",
			want: []diff.Operation{
				{Op: diff.Keep, Char: 'a'},
				{Op: diff.Delete, Char: 'b'},
				{Op: diff.Keep, Char: 'c'},
			},
		},
		{
			s1: "abc",
			s2: "axc",
			want: []diff.Operation{
				{Op: diff.Keep, Char: 'a'},
				{Op: diff.Insert, Char: 'x'},
				{Op: diff.Delete, Char: 'b'},
				{Op: diff.Keep, Char: 'c'},
			},
		},
		{
			s1: "abcd",
			s2: "xabdy",
			want: []diff.Operation{
				{Op: diff.Insert, Char: 'x'},
				{Op: diff.Keep, Char: 'a'},
				{Op: diff.Keep, Char: 'b'},
				{Op: diff.Delete, Char: 'c'},
				{Op: diff.Keep, Char: 'd'},
				{Op: diff.Insert, Char: 'y'},
			},
		},
		{
			s1: "xabdyefg",
			s2: "E",
			want: []diff.Operation{
				{Op: diff.Insert, Char: 'E'},
				{Op: diff.Delete, Char: 'x'},
				{Op: diff.Delete, Char: 'a'},
				{Op: diff.Delete, Char: 'b'},
				{Op: diff.Delete, Char: 'd'},
				{Op: diff.Delete, Char: 'y'},
				{Op: diff.Delete, Char: 'e'},
				{Op: diff.Delete, Char: 'f'},
				{Op: diff.Delete, Char: 'g'},
			},
		},
	}
	ignoreDist := cmpopts.IgnoreFields(diff.Operation{}, "Dist")
	for _, test := range tests {
		got, err := diff.Diff(test.s1, test.s2)
		if err != nil {
			t.Fatalf("diff.Diff(%q, %q): %v", test.s1, test.s2, err)
		}
		if msg := cmp.Diff(test.want, got, ignoreDist); msg != "" {
			t.Errorf("diff.Diff(%q, %q): (-want, +got)\n%s", test.s1, test.s2, msg)
		}
	}
}

func TestDistance(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   int
	}{
		{"", "a", 1},
		{"a", "", 1},
		{"a", "a", 0},
		{"abc", "abc", 0},
		{"ac", "abc", 1},
		{"abc", "ac", 1},
		{"abc", "axc", 2},
		{"abcd", "xabdy", 3},
	}
	for _, test := range tests {
		got, err := diff.Distance(test.s1, test.s2)
		if err != nil {
			t.Fatalf("diff.Distance(%q, %q): %v", test.s1, test.s2, err)
		}
		if got != test.want {
			t.Errorf("diff.Distance(%q, %q): want %d, got %d", test.s1, test.s2, test.want, got)
		}
	}
}

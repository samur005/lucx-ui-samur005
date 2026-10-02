package vkcreator

import "testing"

func TestStripOne(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"abc123", "abc123"},
		{"https://vk.ru/call/join/abc123", "abc123"},
		{"https://vk.com/call/join/xyz?foo=1", "xyz"},
		{"  hash1  ", "hash1"},
		{"https://example.com/other", ""},
	}
	for _, tc := range cases {
		if got := StripOne(tc.in); got != tc.want {
			t.Errorf("StripOne(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeMerge(t *testing.T) {
	if got := Normalize("a, b, a"); got != "a,b" {
		t.Fatalf("Normalize=%q", got)
	}
	if got := MergeHashes("a,b", "c,a"); got != "a,b,c" {
		t.Fatalf("Merge=%q", got)
	}
}

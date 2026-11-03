package cli

import (
	"slices"
	"testing"
)

func TestParseSelection(t *testing.T) {
	for in, want := range map[string][]int{"": nil, "1": {0}, "3, 1 3": {2, 0}, "1,2,3": {0, 1, 2}} {
		got, err := parseSelection(in, 3)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%q: got %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"0", "4", "x", "1,-2"} {
		if _, err := parseSelection(in, 3); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}

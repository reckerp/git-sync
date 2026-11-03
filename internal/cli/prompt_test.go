package cli

import (
	"io"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	for in, want := range map[string]bool{"": true, "\n": true, "y\n": true, "Yes\n": true, "n\n": false, "no": false} {
		got, err := newPrompter(strings.NewReader(in), io.Discard).confirm("ok?")
		if err != nil || got != want {
			t.Errorf("%q: got %v, %v; want %v", in, got, err, want)
		}
	}
}

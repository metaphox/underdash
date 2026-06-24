package display

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerbosef(t *testing.T) {
	var buf bytes.Buffer
	old := verboseOut
	verboseOut = &buf
	defer func() { verboseOut = old; SetVerbose(false) }()

	SetVerbose(false)
	Verbosef("hidden %d", 1)
	if buf.Len() != 0 {
		t.Errorf("verbose off should emit nothing, got %q", buf.String())
	}

	SetVerbose(true)
	Verbosef("backend=%s", "claude")
	if !strings.Contains(buf.String(), "backend=claude") {
		t.Errorf("verbose on should emit message, got %q", buf.String())
	}
}

func TestRedactKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"abcd", "****"},
		{"sk-1234567890", "****7890"},
	}
	for _, tt := range tests {
		if got := RedactKey(tt.in); got != tt.want {
			t.Errorf("RedactKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

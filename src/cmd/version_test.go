package cmd

import (
	"strings"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	info := versionInfo()

	if !strings.Contains(info, version) {
		t.Errorf("versionInfo should contain the version %q", version)
	}
	// It must double as a re-readable data disclosure.
	for _, phrase := range []string{"working directory", "git", "environment variable"} {
		if !strings.Contains(strings.ToLower(info), phrase) {
			t.Errorf("versionInfo should disclose %q; got:\n%s", phrase, info)
		}
	}
}

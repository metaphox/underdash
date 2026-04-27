package backend

import (
	"testing"
)

// --- Finding #3: all spec-defined backend types should be accepted ---

func TestNew_AllSpecTypes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			"stdout",
			Config{Type: "stdout"},
			false, "",
		},
		{
			"claude with key",
			Config{Type: "claude", APIKey: "sk-test"},
			false, "",
		},
		{
			"claude without key",
			Config{Type: "claude"},
			true, "requires an API key",
		},
		{
			"openai should not be unknown",
			Config{Type: "openai"},
			true, "not yet implemented", // should be a friendly "not implemented" not "unknown"
		},
		{
			"local should not be unknown",
			Config{Type: "local"},
			true, "not yet implemented",
		},
		{
			"http should not be unknown",
			Config{Type: "http"},
			true, "not yet implemented",
		},
		{
			"truly unknown type",
			Config{Type: "foobar"},
			true, "unknown backend type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("New(%+v) should return error", tt.cfg)
				}
				if tt.errMsg != "" && !containsStr(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("New(%+v) unexpected error: %v", tt.cfg, err)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

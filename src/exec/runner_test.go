package exec

import (
	"os"
	"testing"

	"metaphox/underdash/response"
)

// withStdin replaces os.Stdin with a pipe carrying the given input for the
// duration of the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})
}

func TestRiskLevelString(t *testing.T) {
	cases := map[RiskLevel]string{
		Safe:      "safe",
		Confirm:   "confirm",
		Dangerous: "dangerous",
		Denied:    "denied",
	}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("RiskLevel(%d).String() = %q, want %q", lvl, got, want)
		}
	}
}

func TestRun_Outcomes(t *testing.T) {
	t.Run("explanation", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Explanation, Explanation: "hi"}, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "explained" {
			t.Errorf("Action = %q, want explained", out.Action)
		}
	})

	t.Run("dry-run command does not execute", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Command, Command: "ls -la"}, false, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "dry-run" || out.Risk != "safe" {
			t.Errorf("Outcome = %+v, want {dry-run safe}", out)
		}
	})

	t.Run("denied command", func(t *testing.T) {
		overrides := &PolicyOverrides{Deny: []string{"rm -rf /"}}
		out, err := Run(&response.Result{Type: response.Command, Command: "rm -rf /"}, true, false, overrides)
		if err == nil {
			t.Fatal("denied command should error")
		}
		if out.Action != "denied" || out.Risk != "denied" {
			t.Errorf("Outcome = %+v, want {denied denied}", out)
		}
	})

	t.Run("autoYes executes", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Command, Command: "true"}, true, false, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Action != "executed" {
			t.Errorf("Action = %q, want executed", out.Action)
		}
	})

	t.Run("unknown response type errors", func(t *testing.T) {
		if _, err := Run(&response.Result{Type: "bogus"}, false, false, nil); err == nil {
			t.Error("expected error for unknown response type")
		}
	})
}

func TestPromptUser(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"", false}, // EOF without input
		{"  y  \n", true},
	}

	for _, tc := range tests {
		t.Run("input "+tc.input, func(t *testing.T) {
			withStdin(t, tc.input)
			if got := promptUser("Execute? "); got != tc.want {
				t.Errorf("promptUser(%q) = %t, want %t", tc.input, got, tc.want)
			}
		})
	}
}

func TestRunCommand_Prompted(t *testing.T) {
	// "touch x" matches no safe command, so it classifies as Confirm.
	t.Run("confirm-risk command declined", func(t *testing.T) {
		withStdin(t, "n\n")
		out, err := Run(&response.Result{Type: response.Command, Command: "touch nonexistent-dir/x"}, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "declined" || out.Risk != "confirm" {
			t.Errorf("Outcome = %+v, want {declined confirm}", out)
		}
	})

	t.Run("dangerous command declined", func(t *testing.T) {
		withStdin(t, "n\n")
		out, err := Run(&response.Result{Type: response.Command, Command: "rm -rf ./x"}, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "declined" || out.Risk != "dangerous" {
			t.Errorf("Outcome = %+v, want {declined dangerous}", out)
		}
	})

	t.Run("confirm-risk command accepted", func(t *testing.T) {
		withStdin(t, "y\n")
		out, err := Run(&response.Result{Type: response.Command, Command: "true"}, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "executed" {
			t.Errorf("Action = %q, want executed", out.Action)
		}
	})
}

func TestRunScript(t *testing.T) {
	script := "#!/bin/sh\nexit 0\n"

	t.Run("dry-run does not execute", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Script, Script: script}, false, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "dry-run" || out.Risk != "dangerous" {
			t.Errorf("Outcome = %+v, want {dry-run dangerous}", out)
		}
	})

	t.Run("declined", func(t *testing.T) {
		withStdin(t, "n\n")
		out, err := Run(&response.Result{Type: response.Script, Script: script}, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Action != "declined" {
			t.Errorf("Action = %q, want declined", out.Action)
		}
	})

	t.Run("autoYes executes", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Script, Script: script}, true, false, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Action != "executed" {
			t.Errorf("Action = %q, want executed", out.Action)
		}
	})

	t.Run("failing script surfaces the error", func(t *testing.T) {
		out, err := Run(&response.Result{Type: response.Script, Script: "#!/bin/sh\nexit 3\n"}, true, false, nil)
		if err == nil {
			t.Fatal("expected error from failing script")
		}
		if out.Action != "executed" {
			t.Errorf("Action = %q, want executed", out.Action)
		}
	})
}

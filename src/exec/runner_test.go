package exec

import (
	"testing"

	"metaphox/underdash/response"
)

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
}

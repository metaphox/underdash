package display

import (
	"fmt"
	"io"
	"os"
)

var (
	verbose    bool
	verboseOut io.Writer = os.Stderr
)

// SetVerbose enables or disables verbose diagnostic output.
func SetVerbose(v bool) { verbose = v }

// Verbosef writes a diagnostic line to stderr, but only when verbose mode is
// on. Output goes to stderr so stdout pipes stay clean. Callers must never pass
// secrets — use RedactKey for anything key-shaped.
func Verbosef(format string, a ...any) {
	if !verbose {
		return
	}
	fmt.Fprintf(verboseOut, "» "+format+"\n", a...)
}

// RedactKey masks all but the last four characters of a secret so it can be
// referenced in diagnostics without disclosing it.
func RedactKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

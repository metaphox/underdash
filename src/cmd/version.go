package cmd

import "fmt"

// version is the build version, overridable via -ldflags "-X .../cmd.version=...".
var version = "dev"

// disclosureText enumerates everything Underdash may transmit to a remote
// backend. It is printed by --version (so users can re-read it without resetting
// consent) and on the first-run acknowledgment prompt.
const disclosureText = `Underdash sends the following to the configured backend when it is remote
(claude / openai / http) — never for the local or stdout backends:

  - OS, architecture, and shell name
  - the current working directory path (may reveal your username and projects)
  - a top-level directory listing (filenames only, capped at 20)
  - git metadata when in a repo: branch, short status (modified/untracked paths),
    recent commit subjects, and the origin remote URL
  - the detected project type and its marker file
  - notable tools found on your $PATH
  - shell history — only when explicitly enabled (context.history: true)
  - the NAMES of environment variables referenced in your prompt (never values)
  - your prompt text verbatim, including anything you paste into it

API keys and environment-variable VALUES are never transmitted or logged.`

// versionInfo returns the version line followed by the full data disclosure.
func versionInfo() string {
	return fmt.Sprintf("underdash %s\n\n%s", version, disclosureText)
}

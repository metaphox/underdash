// Package cmd implements CLI commands for the underdash binary.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"metaphox/underdash/backend"
	"metaphox/underdash/display"
	"metaphox/underdash/exec"
	"metaphox/underdash/input"
	"metaphox/underdash/prompt"
	"metaphox/underdash/response"
	"metaphox/underdash/sysinfo"
)

const maxRetries = 3

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "underdash [flags] [prompt...]",
		Short: "_ ask LLM for help one line at a time",
		Long:  `Underdash is a non-interactive CLI coding agent. Alias it as _ for quick one-shot shell assistance.`,
		// Accept arbitrary args as the prompt.
		Args:          cobra.ArbitraryArgs,
		RunE:          runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeConfig(cmd)
		},
	}
)

// Execute runs the root command and exits the process on error.
func Execute() {
	rootCmd.SetArgs(normalizeLeadingArgs(rootCmd.Flags(), os.Args[1:]))
	if err := rootCmd.Execute(); err != nil {
		renderError(err)
		os.Exit(1)
	}
}

// renderError pretty-prints an error to stderr, recognizing flag-usage errors
// and structured backend.APIError values; anything else prints as a plain
// "error: <msg>" line.
func renderError(err error) {
	if msg, ok := flagErrorHint(err); ok {
		display.ShowError(msg)
		return
	}

	var apiErr *backend.APIError
	if errors.As(err, &apiErr) {
		summary := fmt.Sprintf("%s (%s)", apiErr.Summary(), apiErr.Backend)
		if apiErr.StatusCode != 0 {
			summary = fmt.Sprintf("%s (%s, HTTP %d)", apiErr.Summary(), apiErr.Backend, apiErr.StatusCode)
		}
		var details []string
		if apiErr.Message != "" {
			details = append(details, apiErr.Message)
		}
		if apiErr.RequestID != "" {
			details = append(details, "request id: "+apiErr.RequestID)
		}
		if h := apiErr.Hint(); h != "" {
			details = append(details, "hint: "+h)
		}
		display.ShowErrorDetails(summary, details)
		return
	}

	display.ShowError(err.Error())
}

// flagErrorHint turns pflag's terse "unknown flag" errors into actionable
// guidance, since a prompt CLI user may have meant the token as prompt text.
// Handles both long ("unknown flag: --foo") and shorthand
// ("unknown shorthand flag: 'x' in -x") forms. Returns (message, true) when err
// is an unknown-flag error, else ("", false).
func flagErrorHint(err error) (string, bool) {
	msg := err.Error()

	// Extract the offending token ("--foo" or "-x") from either error form.
	var token string
	switch {
	case strings.HasPrefix(msg, "unknown flag: "):
		token = strings.TrimPrefix(msg, "unknown flag: ")
	case strings.HasPrefix(msg, "unknown shorthand flag: "):
		// e.g. "unknown shorthand flag: 'x' in -xyz" — take what follows " in ".
		if i := strings.LastIndex(msg, " in "); i >= 0 {
			token = msg[i+len(" in "):]
		}
	default:
		return "", false
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", msg)
	fmt.Fprintf(&b, "If you meant a flag, check the spelling (see _ --help for the list).\n")

	switch {
	case strings.HasPrefix(token, "--"):
		// "--these are prompts" was likely meant as "-- these are prompts".
		rest := strings.TrimPrefix(token, "--")
		fmt.Fprintf(&b, "If %q is prompt text, put a space after -- so the rest is taken literally:\n", token)
		fmt.Fprintf(&b, "    _ -- %s ...", rest)
	case strings.HasPrefix(token, "-"):
		// Shorthand: prefix the whole prompt with -- to keep it literal.
		fmt.Fprintf(&b, "If %q is prompt text, prefix the prompt with -- so it is taken literally:\n", token)
		fmt.Fprintf(&b, "    _ -- %s ...", token)
	}
	return b.String(), true
}

// normalizeLeadingArgs makes a leading "-<digit>" token (e.g. "-1") be taken as
// prompt text rather than rejected as an unknown shorthand flag, since a pflag
// shorthand is always a letter — "-1" can never be a valid flag. It inserts a
// "--" separator before the first such token, skipping any leading boolean
// flags so combinations like "-n -1 times" still work. Everything else (a
// bareword, a value-taking flag, or an unknown letter-shorthand like "-x") is
// left for pflag to handle.
func normalizeLeadingArgs(flags *pflag.FlagSet, args []string) []string {
	for i, a := range args {
		switch {
		case a == "--":
			return args // user was already explicit
		case startsWithDashDigit(a):
			out := make([]string, 0, len(args)+1)
			out = append(out, args[:i]...)
			out = append(out, "--")
			out = append(out, args[i:]...)
			return out
		case strings.HasPrefix(a, "-") && isBoolFlag(flags, a):
			continue // skip leading boolean flag, keep scanning
		default:
			return args // bareword, value-flag, or unknown — let pflag decide
		}
	}
	return args
}

// startsWithDashDigit reports whether token is a single dash followed by a
// digit (e.g. "-1", "-1.5", "-3x"), which can never be a valid pflag shorthand.
func startsWithDashDigit(token string) bool {
	return len(token) >= 2 && token[0] == '-' && token[1] >= '0' && token[1] <= '9'
}

// isBoolFlag reports whether token refers only to boolean flags, which never
// consume a following argument. Handles long ("--dry-run") and single or
// combined shorthand ("-n", "-ny") forms.
func isBoolFlag(flags *pflag.FlagSet, token string) bool {
	if strings.HasPrefix(token, "--") {
		name := strings.SplitN(strings.TrimPrefix(token, "--"), "=", 2)[0]
		f := flags.Lookup(name)
		return f != nil && f.Value.Type() == "bool"
	}
	// Single-dash; an "=" form (e.g. "-n=false") may consume a value, so skip it.
	if strings.Contains(token, "=") {
		return false
	}
	chars := strings.TrimPrefix(token, "-")
	if chars == "" {
		return false
	}
	for _, c := range chars {
		f := flags.ShorthandLookup(string(c))
		if f == nil || f.Value.Type() != "bool" {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.config/underdash/config.yaml)")
	registerRootFlags(rootCmd)
}

// registerRootFlags declares the prompt-command flags. Extracted so tests can
// build a faithful copy of the command's parsing behavior.
func registerRootFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("backend", "b", "", "backend to use (default from config)")
	cmd.Flags().BoolP("dry-run", "n", false, "show the generated command without executing")
	cmd.Flags().Bool("no-exec", false, "alias for --dry-run")
	cmd.Flags().BoolP("yes", "y", false, "skip all confirmations")
	cmd.Flags().String("output", "", "output mode: streaming or plain (default: streaming)")

	// Stop parsing flags once the first bareword (the start of the prompt) is
	// seen, so flags only count before the prompt. A standalone "--" still
	// forces everything after it into the prompt verbatim (pflag built-in),
	// letting users write prompts that start with a dash.
	cmd.Flags().SetInterspersed(false)
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	// Set output mode: CLI flag wins, falling back to config.
	outputMode, _ := cmd.Flags().GetString("output") // err is nil; flag registered in init()
	if outputMode == "" {
		outputMode = viper.GetString("output.mode")
	}
	if outputMode != "" {
		display.SetOutputMode(outputMode)
	}

	// Resolve dry-run: either --dry-run or --no-exec.
	dryRun, _ := cmd.Flags().GetBool("dry-run") // err is nil; flag registered in init()
	noExec, _ := cmd.Flags().GetBool("no-exec") // err is nil; flag registered in init()
	dryRun = dryRun || noExec

	// 1. Parse input. ArgsLenAtDash() locates a "--" the flag parser consumed;
	// input.Parse also handles a literal "--" left in args.
	inp := input.Parse(args, cmd.ArgsLenAtDash())

	// 2. Gather context.
	sysCtx := sysinfo.Gather()

	// 3. Build prompt.
	sysProm := prompt.BuildSystemPrompt()
	ctxBlock := prompt.BuildContextBlock(sysCtx, inp)
	userMsg := prompt.BuildUserMessage(inp)
	fullSystemPrompt := sysProm + "\n\n" + ctxBlock

	// 4. Resolve backend.
	be, err := resolveBackend(cmd)
	if err != nil {
		return err
	}

	// 5. Send to backend with retry loop.
	resp, err := sendWithRetry(cmd.Context(), be, fullSystemPrompt, userMsg)
	if err != nil {
		return err
	}

	// stdout backend returns empty — nothing to execute.
	if resp == nil {
		return nil
	}

	// 6. Execute / display result.
	autoYes, _ := cmd.Flags().GetBool("yes") // err is nil; flag registered in init()
	return exec.Run(resp, autoYes, dryRun, loadPolicyOverrides())
}

// loadPolicyOverrides reads execution.auto_run / execution.confirm / execution.deny
// from Viper. Returns nil if no overrides are configured.
func loadPolicyOverrides() *exec.PolicyOverrides {
	autoRun := viper.GetStringSlice("execution.auto_run")
	confirm := viper.GetStringSlice("execution.confirm")
	deny := viper.GetStringSlice("execution.deny")
	if len(autoRun) == 0 && len(confirm) == 0 && len(deny) == 0 {
		return nil
	}
	return &exec.PolicyOverrides{
		AutoRun: autoRun,
		Confirm: confirm,
		Deny:    deny,
	}
}

func sendWithRetry(ctx context.Context, be backend.Backend, systemPrompt string, userMsg string) (*response.Result, error) {
	// Start spinner.
	spin := display.NewSpinner()
	spin.Start("Thinking...")

	raw, err := be.Send(ctx, backend.Request{
		SystemPrompt: systemPrompt,
		UserMessage:  userMsg,
		MaxTokens:    2048,
	})
	spin.Stop()

	if err != nil {
		return nil, err
	}

	// stdout backend returns empty string.
	if raw == "" {
		return nil, nil
	}

	// Try to parse.
	resp, parseErr := response.Parse(raw)
	if parseErr == nil {
		return resp, nil
	}

	// Retry loop.
	if !response.IsRetryable(raw) {
		// Clearly not JSON — show raw output as fallback.
		fmt.Fprintln(os.Stderr, "Model did not return JSON. Raw response:")
		fmt.Println(raw)
		return nil, fmt.Errorf("model returned non-JSON response")
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		retryMsg := prompt.BuildRetryMessage(raw)
		combinedUserMsg := userMsg + "\n\n" + retryMsg

		spin = display.NewSpinner()
		spin.Start(fmt.Sprintf("Retrying (%d/%d)...", attempt, maxRetries))

		raw, err = be.Send(ctx, backend.Request{
			SystemPrompt: systemPrompt,
			UserMessage:  combinedUserMsg,
			MaxTokens:    2048,
		})
		spin.Stop()

		if err != nil {
			return nil, err
		}

		resp, parseErr = response.Parse(raw)
		if parseErr == nil {
			return resp, nil
		}

		if !response.IsRetryable(raw) {
			break
		}
	}

	// Final failure — show raw response.
	fmt.Fprintln(os.Stderr, "Failed to get valid JSON after retries. Raw response:")
	fmt.Println(raw)
	return nil, fmt.Errorf("model did not return valid JSON after %d retries", maxRetries)
}

func resolveBackend(cmd *cobra.Command) (backend.Backend, error) {
	backendName, _ := cmd.Flags().GetString("backend") // err is nil; flag registered in init()

	// If not specified via flag, check config.
	if backendName == "" {
		backendName = viper.GetString("default_backend")
	}
	if backendName == "" {
		backendName = "claude" // ultimate default
	}

	// Build config for the backend.
	cfg := backend.Config{
		Type: backendName,
	}

	// For named backends, read from the backends.X config section.
	prefix := "backends." + backendName + "."
	if viper.IsSet(prefix + "type") {
		cfg.Type = viper.GetString(prefix + "type")
	}
	cfg.Model = viper.GetString(prefix + "model")
	cfg.Endpoint = viper.GetString(prefix + "endpoint")
	cfg.APIKey = viper.GetString(prefix + "api_key")

	// Check env_key: if set, use the referenced env var for the API key.
	// Falls back to a per-type default (e.g. ANTHROPIC_API_KEY for claude) so
	// the env var works even without a config file declaring env_key.
	envKey := viper.GetString(prefix + "env_key")
	if envKey == "" {
		envKey = defaultEnvKey(cfg.Type)
	}
	if envKey != "" {
		if v := os.Getenv(envKey); v != "" {
			cfg.APIKey = v
		}
	}

	return backend.New(cfg)
}

func defaultEnvKey(backendType string) string {
	switch backendType {
	case "claude":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	}
	return ""
}

func initializeConfig(cmd *cobra.Command) error {
	// Load .env from the binary's directory before reading config or env,
	// so keys defined there are visible to viper and backend resolution.
	loadDotEnv()

	viper.SetEnvPrefix("UNDERDASH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home + "/.config/underdash")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	err := viper.ReadInConfig()
	if err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}

		return viper.BindPFlags(cmd.Flags())
	}

	// Config file loaded — check permissions.
	cfgPath := viper.ConfigFileUsed()
	if warning := checkConfigPermissions(cfgPath); warning != "" {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}

	return viper.BindPFlags(cmd.Flags())
}

// checkConfigPermissions checks if a config file containing api_key values
// is world-readable and returns a warning message if so.
func checkConfigPermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	mode := info.Mode().Perm()
	// Check if group or others can read (0044 = group-read + other-read).
	if mode&0044 == 0 {
		return "" // not world-readable, fine
	}

	// Check if the file contains any api_key.
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	if strings.Contains(string(content), "api_key") {
		return fmt.Sprintf("config file %s is world-readable and contains API keys. Consider: chmod 600 %s", path, path)
	}

	return ""
}

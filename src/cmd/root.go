// Package cmd implements CLI commands for the underdash binary.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"metaphox/underdash/attach"
	"metaphox/underdash/audit"
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

// Execute runs the root command and exits the process on error. The root
// context is cancelled on SIGINT/SIGTERM so an in-flight backend request or
// stream aborts cleanly; a second signal force-quits.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd.SetArgs(normalizeLeadingArgs(rootCmd.Flags(), os.Args[1:]))
	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return
	}

	// Render the error, then exit with the code from the stable contract.
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(os.Stderr, "cancelled")
	case isExitError(err):
		// The command already emitted its own output; don't double-report.
	default:
		renderError(err)
	}
	os.Exit(exitCodeFor(err))
}

// renderError pretty-prints an error to stderr, recognizing flag-usage errors
// and structured backend.APIError values; anything else prints as a plain
// "error: <msg>" line.
func renderError(err error) {
	if msg, ok := flagErrorHint(err); ok {
		display.ShowError(msg)
		return
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		display.ShowError("backend timed out — check your network connection or the backend endpoint")
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
	cmd.Flags().BoolP("verbose", "v", false, "print diagnostics to stderr")
	cmd.Flags().Bool("version", false, "print version and data disclosure")
	cmd.Flags().Bool("sysinfo", false, "print gathered system information (spec format) and exit")

	// Stop parsing flags once the first bareword (the start of the prompt) is
	// seen, so flags only count before the prompt. A standalone "--" still
	// forces everything after it into the prompt verbatim (pflag built-in),
	// letting users write prompts that start with a dash.
	cmd.Flags().SetInterspersed(false)
}

func runRoot(cmd *cobra.Command, args []string) error {
	// --version short-circuits everything, even with no prompt.
	if v, _ := cmd.Flags().GetBool("version"); v {
		fmt.Println(versionInfo())
		return nil
	}

	if vb, _ := cmd.Flags().GetBool("verbose"); vb {
		display.SetVerbose(true)
	}

	// --sysinfo prints the gathered context spec and exits, even with no prompt.
	if si, _ := cmd.Flags().GetBool("sysinfo"); si {
		sysCtx := sysinfo.Gather()
		fmt.Println(prompt.BuildContextBlock(sysCtx, &input.ParsedInput{}, nil))
		return nil
	}

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
	autoYes, _ := cmd.Flags().GetBool("yes") // err is nil; flag registered in init()

	// 1. Parse input. ArgsLenAtDash() locates a "--" the flag parser consumed;
	// input.Parse also handles a literal "--" left in args.
	inp := input.Parse(args, cmd.ArgsLenAtDash())

	// 1b. Load any @file attachments, failing fast (before any backend call) on
	// unreadable, oversized, or unsupported files.
	atts, err := attach.Load(inp.Attachments)
	if err != nil {
		return err
	}
	for _, a := range atts {
		display.Verbosef("attachment: %s (%s, %s)", a.Filename, a.Kind, a.MediaType)
	}

	// 2. Gather context.
	sysCtx := sysinfo.Gather()
	display.Verbosef("context: cwd=%s git=%t project=%s tools=%d history=%d",
		sysCtx.CWD, sysCtx.InGitRepo, sysCtx.ProjectType, len(sysCtx.PathTools), len(sysCtx.ShellHistory))

	// 3. Build prompt.
	sysProm := prompt.BuildSystemPrompt()
	ctxBlock := prompt.BuildContextBlock(sysCtx, inp, atts)
	userMsg := prompt.BuildUserMessage(inp)
	fullSystemPrompt := sysProm + "\n\n" + ctxBlock

	// 4. Resolve backend (resolves/discovers the model too).
	be, cfg, err := resolveBackend(cmd.Context(), cmd)
	if err != nil {
		return err
	}
	display.Verbosef("backend: %s model=%s", be.Name(), cfg.Model)
	display.Verbosef("system prompt:\n%s", fullSystemPrompt)
	display.Verbosef("user message:\n%s", userMsg)

	// 4b. First-run privacy acknowledgment for remote backends.
	if err := ensureConsent(be.Name(), autoYes); err != nil {
		return err
	}

	// 5. Send to backend with retry loop, self-healing a retired model once.
	// detail is the spinner's status line: where the request is going and what
	// local context it carries.
	detail := modelLabel(be, cfg.Model)
	if cs := contextSummary(sysCtx, len(atts)); cs != "" {
		detail += " · ctx: " + cs
	}
	start := time.Now()
	resp, err := sendWithRetry(cmd.Context(), be, fullSystemPrompt, userMsg, atts, detail)
	if err != nil {
		if healedBe, healed := maybeSelfHeal(cmd.Context(), cmd, &cfg, be, err); healed {
			be = healedBe
			detail = modelLabel(be, cfg.Model)
			if cs := contextSummary(sysCtx, len(atts)); cs != "" {
				detail += " · ctx: " + cs
			}
			resp, err = sendWithRetry(cmd.Context(), be, fullSystemPrompt, userMsg, atts, detail)
		}
	}
	elapsed := time.Since(start).Milliseconds()
	display.Verbosef("backend responded in %dms", elapsed)
	if err != nil {
		recordAudit(inp, be, nil, exec.Outcome{Action: "error"}, err, elapsed)
		return err
	}

	// stdout backend returns empty — nothing to execute.
	if resp == nil {
		return nil
	}

	// 6. Execute / display result.
	outcome, runErr := exec.Run(resp, autoYes, dryRun, loadPolicyOverrides())
	display.Verbosef("result: type=%s action=%s risk=%s", resp.Type, outcome.Action, outcome.Risk)
	recordAudit(inp, be, resp, outcome, runErr, elapsed)
	return runErr
}

// recordAudit writes one audit record (best-effort; a log failure only warns).
func recordAudit(inp *input.ParsedInput, be backend.Backend, resp *response.Result, outcome exec.Outcome, err error, durationMs int64) {
	query := inp.Query
	if query == "" {
		query = inp.SupplementaryPrompt
	}
	rec := audit.Record{
		Query:      query,
		Backend:    be.Name(),
		Model:      viper.GetString("backends." + be.Name() + ".model"),
		Action:     outcome.Action,
		Risk:       outcome.Risk,
		Exit:       exitCodeFor(err),
		DurationMs: durationMs,
	}
	if resp != nil {
		rec.ResponseType = string(resp.Type)
		rec.Command = resp.Command
	}
	if err != nil {
		rec.Error = err.Error()
	}
	if logErr := audit.Log(auditConfig(), rec); logErr != nil {
		fmt.Fprintln(os.Stderr, "warning: audit log:", logErr)
	}
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

// modelLabel renders the backend and model compactly for the spinner, e.g.
// "claude/opus-4-8", "openai/gpt-4o", "local/llama3". A leading "<backend>-" on
// the model id is trimmed to avoid "claude/claude-opus-4-8". When the model is
// not yet known (e.g. the stdout backend), just the backend name is shown.
func modelLabel(be backend.Backend, model string) string {
	if model == "" {
		return be.Name()
	}
	return be.Name() + "/" + strings.TrimPrefix(model, be.Name()+"-")
}

// contextSummary lists which local context signals are being sent, for the
// spinner — e.g. "git, go, 14 tools, 2 files". Empty when nothing was gathered.
func contextSummary(sysCtx *sysinfo.SystemContext, attCount int) string {
	var parts []string
	if sysCtx.InGitRepo {
		parts = append(parts, "git")
	}
	if sysCtx.ProjectType != "" {
		parts = append(parts, sysCtx.ProjectType)
	}
	if n := len(sysCtx.PathTools); n > 0 {
		parts = append(parts, fmt.Sprintf("%d tools", n))
	}
	if len(sysCtx.ShellHistory) > 0 {
		parts = append(parts, "history")
	}
	if attCount > 0 {
		noun := "files"
		if attCount == 1 {
			noun = "file"
		}
		parts = append(parts, fmt.Sprintf("%d %s", attCount, noun))
	}
	return strings.Join(parts, ", ")
}

func sendWithRetry(ctx context.Context, be backend.Backend, systemPrompt string, userMsg string, atts []backend.Attachment, detail string) (*response.Result, error) {
	// Start spinner.
	spin := display.NewSpinner()
	spin.Start("Thinking", detail, time.Now().Add(backend.ResponseHeaderTimeout))

	raw, err := be.Send(ctx, backend.Request{
		SystemPrompt:    systemPrompt,
		UserMessage:     userMsg,
		MaxTokens:       2048,
		Attachments:     atts,
		OnResponseStart: spin.ClearDeadline,
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
		spin.Start(fmt.Sprintf("Retrying (%d/%d)", attempt, maxRetries), detail, time.Now().Add(backend.ResponseHeaderTimeout))

		raw, err = be.Send(ctx, backend.Request{
			SystemPrompt:    systemPrompt,
			UserMessage:     combinedUserMsg,
			MaxTokens:       2048,
			Attachments:     atts,
			OnResponseStart: spin.ClearDeadline,
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

// selectedBackendName resolves the backend (config-section) name from the flag,
// then config default, then the ultimate "claude" fallback.
func selectedBackendName(cmd *cobra.Command) string {
	name, _ := cmd.Flags().GetString("backend") // err is nil; flag registered in init()
	if name == "" {
		name = viper.GetString("default_backend")
	}
	if name == "" {
		name = "claude" // ultimate default
	}
	return name
}

func resolveBackend(ctx context.Context, cmd *cobra.Command) (backend.Backend, backend.Config, error) {
	backendName := selectedBackendName(cmd)

	// Build config for the backend.
	cfg := backend.Config{Type: backendName}

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

	be, err := backend.New(cfg)
	if err != nil {
		return nil, cfg, err
	}

	// Resolve the model dynamically when none is configured — model IDs are not
	// hardcoded because providers retire them.
	if cfg.Model == "" {
		if lister, ok := be.(backend.ModelLister); ok {
			autoYes, _ := cmd.Flags().GetBool("yes")
			path := configFilePath()
			writable := configWritable(path)
			interactive := writable && display.IsTTY() && !autoYes

			model, derr := resolveModelFor(ctx, lister, cfg.Type, interactive)
			if derr != nil {
				// claude/openai must have a model; local/http can let the server decide.
				if cfg.Type == "claude" || cfg.Type == "openai" {
					return nil, cfg, fmt.Errorf("couldn't determine a model for %s (%w); set backends.%s.model in your config", cfg.Type, derr, backendName)
				}
			} else {
				cfg.Model = model
				persistDiscoveredModel(path, backendName, model, writable)
				if be, err = backend.New(cfg); err != nil {
					return nil, cfg, err
				}
			}
		}
	}

	return be, cfg, nil
}

// persistDiscoveredModel saves a freshly discovered model, warning (but
// continuing) if the config can't be written.
func persistDiscoveredModel(path, backendName, model string, writable bool) {
	if writable {
		if err := persistModel(path, backendName, model); err != nil {
			display.ShowError(fmt.Sprintf("couldn't save model to %s (%v); using %s for now — set backends.%s.model to persist", path, err, model, backendName))
		}
		return
	}
	display.ShowError(fmt.Sprintf("couldn't save model to %s (not writable); using %s for now — set backends.%s.model to persist", path, model, backendName))
}

// maybeSelfHeal handles a model-not-found 404 by re-discovering a model,
// updating config, and returning a rebuilt backend to retry with.
func maybeSelfHeal(ctx context.Context, cmd *cobra.Command, cfg *backend.Config, be backend.Backend, sendErr error) (backend.Backend, bool) {
	var apiErr *backend.APIError
	if !errors.As(sendErr, &apiErr) || !apiErr.IsModelNotFound() {
		return be, false
	}
	lister, ok := be.(backend.ModelLister)
	if !ok {
		return be, false
	}
	// Auto-pick a replacement; we're mid-request, so don't prompt.
	newModel, derr := resolveModelFor(ctx, lister, cfg.Type, false)
	if derr != nil || newModel == "" || newModel == cfg.Model {
		return be, false
	}

	fmt.Fprintf(os.Stderr, "note: model %q is unavailable; switched to %q\n", cfg.Model, newModel)
	cfg.Model = newModel
	if path := configFilePath(); configWritable(path) {
		_ = persistModel(path, selectedBackendName(cmd), newModel)
	}
	newBe, err := backend.New(*cfg)
	if err != nil {
		return be, false
	}
	return newBe, true
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

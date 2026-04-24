/*
Copyright © 2026 Tao Wu <foxisme@gmail.com>
*/

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"metaphox/underdash/backend"
	uctx "metaphox/underdash/context"
	"metaphox/underdash/display"
	"metaphox/underdash/exec"
	"metaphox/underdash/input"
	"metaphox/underdash/prompt"
	"metaphox/underdash/response"
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.config/underdash/config.yaml)")
	rootCmd.Flags().StringP("backend", "b", "", "backend to use (default from config)")
	rootCmd.Flags().BoolP("dry-run", "n", false, "print the assembled prompt without calling any backend")
	rootCmd.Flags().BoolP("yes", "y", false, "skip all confirmations")
	rootCmd.Flags().String("output", "", "output mode: streaming or plain (default: streaming)")
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	// 1. Parse input.
	inp := input.Parse(args)

	// 2. Gather context.
	sysCtx := uctx.Gather()

	// 3. Build prompt.
	sysProm := prompt.BuildSystemPrompt()
	ctxBlock := prompt.BuildContextBlock(sysCtx, inp)
	userMsg := prompt.BuildUserMessage(inp)
	fullSystemPrompt := sysProm + "\n\n" + ctxBlock

	// 4. Handle dry-run.
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		display.ShowDryRun(fullSystemPrompt, userMsg)
		return nil
	}

	// 5. Resolve backend.
	be, err := resolveBackend(cmd)
	if err != nil {
		return err
	}

	// 6. Send to backend with retry loop.
	autoYes, _ := cmd.Flags().GetBool("yes")
	resp, err := sendWithRetry(cmd.Context(), be, fullSystemPrompt, userMsg)
	if err != nil {
		return err
	}

	// stdout backend returns empty — nothing to execute.
	if resp == nil {
		return nil
	}

	// 7. Execute / display result.
	return exec.Run(resp, autoYes, false)
}

func sendWithRetry(ctx context.Context, be backend.Backend, systemPrompt string, userMsg string) (*response.LLMResponse, error) {
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
	backendName, _ := cmd.Flags().GetString("backend")

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
	envKey := viper.GetString(prefix + "env_key")
	if envKey != "" {
		if v := os.Getenv(envKey); v != "" {
			cfg.APIKey = v
		}
	}

	return backend.New(cfg)
}

func initializeConfig(cmd *cobra.Command) error {
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

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}
	}

	return viper.BindPFlags(cmd.Flags())
}

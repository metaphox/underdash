package cmd

import (
	"fmt"
	"os"
	"strings"
)

// resolveAPIKey determines the API key for a backend from up to three sources,
// in order of decreasing precedence:
//
//  1. the environment variable named by envKey (also fed by the .env loader),
//  2. the file named by keyFile (api_key_file), and
//  3. the inline api_key string.
//
// The first source that yields a value wins. envKey is the already-resolved
// variable name (an explicit env_key or the per-type default). A configured
// api_key_file that cannot be read or that yields no usable key is a hard error,
// so a misconfigured path fails loudly instead of silently falling through.
func resolveAPIKey(inlineKey, keyFile, envKey string) (string, error) {
	if envKey != "" {
		if v := os.Getenv(envKey); v != "" {
			return v, nil
		}
	}
	if keyFile != "" {
		return readAPIKeyFile(keyFile, envKey)
	}
	return inlineKey, nil
}

// readAPIKeyFile reads an API key from path. The file is either a single value
// holding the raw key, or dotenv-style `NAME=value` pairs so one file can serve
// several backends; in the pairs case envKey selects which entry to use. A
// leading "~/" in path is expanded against the user's home directory.
func readAPIKeyFile(path, envKey string) (string, error) {
	expanded := expandHome(path)
	data, err := os.ReadFile(expanded)
	if err != nil {
		return "", fmt.Errorf("read api_key_file %s: %w", path, err)
	}

	pairs := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := parseDotEnvLine(line); ok {
			pairs[k] = v
		}
	}

	// NAME=value form: pick the entry the backend's env_key points at.
	if len(pairs) > 0 {
		if envKey == "" {
			return "", fmt.Errorf("api_key_file %s holds NAME=value pairs but no env_key is set to select one", path)
		}
		v, ok := pairs[envKey]
		if !ok {
			return "", fmt.Errorf("api_key_file %s has no entry for %s", path, envKey)
		}
		if v == "" {
			return "", fmt.Errorf("api_key_file %s has an empty value for %s", path, envKey)
		}
		return v, nil
	}

	// Otherwise the whole file is a single raw key.
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("api_key_file %s is empty", path)
	}
	return key, nil
}

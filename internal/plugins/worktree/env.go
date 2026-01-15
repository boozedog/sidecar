package worktree

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// worktreeEnvFile is the name of the per-repo environment override file.
const worktreeEnvFile = ".worktree-env"

// DefaultEnvOverrides contains environment variables to clear/override for worktree isolation.
// These prevent common issues when running commands in worktrees.
var DefaultEnvOverrides = map[string]string{
	// Go - disable workspace to avoid duplicate module errors
	"GOWORK": "off",

	// Go - clear to prevent inheriting parent build flags
	"GOFLAGS": "",

	// Node.js - clear to prevent inheriting options
	"NODE_OPTIONS": "",
	"NODE_PATH":    "",

	// Python - clear to prevent import path conflicts
	"PYTHONPATH":  "",
	"VIRTUAL_ENV": "",
}

// parseWorktreeEnvFile reads a .worktree-env file and returns key=value pairs.
// Returns empty map if file doesn't exist. Skips comments (lines starting with #).
func parseWorktreeEnvFile(mainRepoPath string) (map[string]string, error) {
	envPath := filepath.Join(mainRepoPath, worktreeEnvFile)

	file, err := os.Open(envPath)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	overrides := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if key != "" {
			overrides[key] = value
		}
	}

	return overrides, scanner.Err()
}

// BuildEnvOverrides returns the combined environment overrides for a worktree.
// User overrides from .worktree-env take precedence over defaults.
func BuildEnvOverrides(mainRepoPath string) map[string]string {
	// Start with defaults
	result := make(map[string]string, len(DefaultEnvOverrides))
	for k, v := range DefaultEnvOverrides {
		result[k] = v
	}

	// Merge user overrides (user values take precedence)
	userOverrides, err := parseWorktreeEnvFile(mainRepoPath)
	if err == nil {
		for k, v := range userOverrides {
			result[k] = v
		}
	}

	return result
}

// GenerateExportCommands generates shell commands to apply environment overrides.
// Empty values generate unset commands, non-empty generate export commands.
func GenerateExportCommands(overrides map[string]string) []string {
	var cmds []string
	for key, value := range overrides {
		if value == "" {
			cmds = append(cmds, "unset "+key)
		} else {
			// Quote the value for shell safety
			cmds = append(cmds, "export "+key+"="+shellQuote(value))
		}
	}
	return cmds
}

// ApplyEnvOverrides applies overrides to an existing environment slice.
// Returns a new slice with overrides applied.
func ApplyEnvOverrides(baseEnv []string, overrides map[string]string) []string {
	// Build map of existing env vars for easy lookup
	envMap := make(map[string]string, len(baseEnv))
	for _, entry := range baseEnv {
		idx := strings.Index(entry, "=")
		if idx != -1 {
			envMap[entry[:idx]] = entry[idx+1:]
		}
	}

	// Apply overrides
	for key, value := range overrides {
		if value == "" {
			delete(envMap, key) // Remove/unset
		} else {
			envMap[key] = value
		}
	}

	// Convert back to slice
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}

	return result
}

// shellQuote quotes a string for safe use in shell commands.
func shellQuote(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

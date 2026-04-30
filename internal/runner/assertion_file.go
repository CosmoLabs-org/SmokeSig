package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// CheckEnvExists verifies that an environment variable is set (non-empty).
func CheckEnvExists(name string) AssertionResult {
	value := os.Getenv(name)
	return AssertionResult{
		Type:     "env_exists",
		Expected: name,
		Actual:   value,
		Passed:   value != "",
	}
}

// CheckFileExists verifies that a file exists at the given path.
// Relative paths are resolved against configDir using filepath.Join.
func CheckFileExists(path, configDir string) AssertionResult {
	resolved := path
	if !filepath.IsAbs(path) {
		resolved = filepath.Join(configDir, path)
	}

	_, err := os.Stat(resolved)
	passed := err == nil

	return AssertionResult{
		Type:     "file_exists",
		Expected: resolved,
		Actual:   resolved,
		Passed:   passed,
	}
}

func CheckFileSize(check *schema.FileSizeCheck, configDir string) AssertionResult {
	resolved := check.Path
	if !filepath.IsAbs(check.Path) {
		resolved = filepath.Join(configDir, check.Path)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return AssertionResult{
			Type:     "file_size",
			Expected: fmt.Sprintf("file %s exists", resolved),
			Actual:   fmt.Sprintf("stat error: %v", err),
			Passed:   false,
		}
	}

	size := info.Size()

	// No thresholds — just check existence (file already stat'd successfully)
	if check.MinBytes == nil && check.MaxBytes == nil {
		return AssertionResult{
			Type:     "file_size",
			Expected: fmt.Sprintf("%s exists", resolved),
			Actual:   fmt.Sprintf("%s (%s)", resolved, formatBytes(size)),
			Passed:   true,
		}
	}

	if check.MinBytes != nil && size < *check.MinBytes {
		return AssertionResult{
			Type:     "file_size",
			Expected: fmt.Sprintf(">=%s", formatBytes(*check.MinBytes)),
			Actual:   formatBytes(size),
			Passed:   false,
		}
	}

	if check.MaxBytes != nil && size > *check.MaxBytes {
		return AssertionResult{
			Type:     "file_size",
			Expected: fmt.Sprintf("<=%s", formatBytes(*check.MaxBytes)),
			Actual:   formatBytes(size),
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "file_size",
		Expected: fileSizeRange(check.MinBytes, check.MaxBytes),
		Actual:   formatBytes(size),
		Passed:   true,
	}
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	const kb = 1024
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func fileSizeRange(min, max *int64) string {
	if min != nil && max != nil {
		return fmt.Sprintf("%s-%s", formatBytes(*min), formatBytes(*max))
	}
	if min != nil {
		return fmt.Sprintf(">=%s", formatBytes(*min))
	}
	return fmt.Sprintf("<=%s", formatBytes(*max))
}

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckCredential verifies a credential is accessible without leaking its value.
func CheckCredential(check *schema.CredentialCheck, configDir string) AssertionResult {
	switch check.Source {
	case "env":
		return checkEnvCredential(check)
	case "file":
		return checkFileCredential(check, configDir)
	case "exec":
		return checkExecCredential(check)
	default:
		return AssertionResult{
			Type:     "credential_check",
			Expected: "source: env|file|exec",
			Actual:   fmt.Sprintf("invalid source %q", check.Source),
			Passed:   false,
		}
	}
}

func checkEnvCredential(check *schema.CredentialCheck) AssertionResult {
	value := os.Getenv(check.Name)
	if value == "" {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("env:%s exists", check.Name),
			Actual:   "not set",
			Passed:   false,
		}
	}

	if check.Contains != "" && !strings.Contains(value, check.Contains) {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("env:%s contains %q", check.Name, check.Contains),
			Actual:   "***redacted***",
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "credential_check",
		Expected: fmt.Sprintf("env:%s exists", check.Name),
		Actual:   "***redacted***",
		Passed:   true,
	}
}

func checkFileCredential(check *schema.CredentialCheck, configDir string) AssertionResult {
	path := check.Name
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("file:%s readable", check.Name),
			Actual:   fmt.Sprintf("error: %v", err),
			Passed:   false,
		}
	}

	if check.Contains != "" && !strings.Contains(string(data), check.Contains) {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("file:%s contains %q", check.Name, check.Contains),
			Actual:   "***redacted***",
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "credential_check",
		Expected: fmt.Sprintf("file:%s readable", check.Name),
		Actual:   "***redacted***",
		Passed:   true,
	}
}

func checkExecCredential(check *schema.CredentialCheck) AssertionResult {
	cmd := exec.Command("sh", "-c", check.Name)
	output, err := cmd.Output()
	if err != nil {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("exec:%q succeeds", check.Name),
			Actual:   fmt.Sprintf("exit error: %v", err),
			Passed:   false,
		}
	}

	if check.Contains != "" && !strings.Contains(string(output), check.Contains) {
		return AssertionResult{
			Type:     "credential_check",
			Expected: fmt.Sprintf("exec:%q output contains %q", check.Name, check.Contains),
			Actual:   "***redacted***",
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "credential_check",
		Expected: fmt.Sprintf("exec:%q succeeds", check.Name),
		Actual:   "***redacted***",
		Passed:   true,
	}
}

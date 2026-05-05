package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckVersion runs a shell command and regex-matches stdout.
func CheckVersion(check *schema.VersionCheck) AssertionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", check.Command)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return AssertionResult{
			Type:     "version_check",
			Expected: fmt.Sprintf("pattern %q", check.Pattern),
			Actual:   fmt.Sprintf("command failed: %v", err),
			Passed:   false,
		}
	}

	re := regexp.MustCompile(check.Pattern)
	output := strings.TrimSpace(stdout.String())
	if re.MatchString(output) {
		return AssertionResult{
			Type:     "version_check",
			Expected: fmt.Sprintf("pattern %q", check.Pattern),
			Actual:   output,
			Passed:   true,
		}
	}
	return AssertionResult{
		Type:     "version_check",
		Expected: fmt.Sprintf("pattern %q", check.Pattern),
		Actual:   fmt.Sprintf("output %q did not match", output),
		Passed:   false,
	}
}

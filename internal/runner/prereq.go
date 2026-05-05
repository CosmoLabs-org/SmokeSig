package runner

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// PrereqResult holds the outcome of a single prerequisite check.
type PrereqResult struct {
	Name   string
	Passed bool
	Output string // first line of stdout (e.g. "go1.26.2")
	Hint   string // from prerequisite config
	Error  error  // non-nil if command exited non-zero or timed out
}

// CheckPrerequisites runs each prerequisite check and returns a result for every
// entry. It never aborts early — the caller decides what to do with failures.
func CheckPrerequisites(prereqs []schema.Prerequisite, timeout time.Duration) []PrereqResult {
	results := make([]PrereqResult, 0, len(prereqs))

	for _, p := range prereqs {
		results = append(results, runPrereq(p, timeout))
	}

	return results
}

// runPrereq executes a single prerequisite check via "sh -c <check>".
func runPrereq(p schema.Prerequisite, timeout time.Duration) PrereqResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", p.Check)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()

	result := PrereqResult{
		Name:   p.Name,
		Hint:   p.Hint,
		Passed: err == nil,
		Error:  err,
	}

	// Capture only the first non-empty line of stdout for display.
	if stdout.Len() > 0 {
		scanner := bufio.NewScanner(&stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				result.Output = line
				break
			}
		}
	}

	return result
}

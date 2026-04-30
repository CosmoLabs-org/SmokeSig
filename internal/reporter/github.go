package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// GitHubActions reports results as GitHub Actions workflow commands and
// markdown step summaries.
type GitHubActions struct {
	w     io.Writer
	tests []TestResultData
}

// NewGitHubActions creates a reporter that writes workflow commands to w
// and a markdown summary to $GITHUB_STEP_SUMMARY (or w as fallback).
func NewGitHubActions(w io.Writer) *GitHubActions {
	return &GitHubActions{w: w}
}

func (g *GitHubActions) PrereqStart(_ string)       {}
func (g *GitHubActions) PrereqResult(_ PrereqResultData) {}
func (g *GitHubActions) TestStart(_ string)          {}

func (g *GitHubActions) TestResult(r TestResultData) {
	g.tests = append(g.tests, r)
}

func (g *GitHubActions) Summary(s SuiteResultData) {
	md := g.buildMarkdown(s)
	g.writeSummary(md)
	g.emitCommands(s)
}

func (g *GitHubActions) buildMarkdown(s SuiteResultData) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Smoke Test Results\n")
	fmt.Fprintf(&b, "**Status**: %s %d/%d passed | Duration: %s\n\n", statusIcon(s.Failed == 0), s.Passed, s.Total, formatGhaDuration(s.Duration))

	if len(g.tests) > 0 {
		fmt.Fprintf(&b, "| Test | Status | Duration |\n")
		fmt.Fprintf(&b, "|------|--------|----------|\n")
		for _, t := range g.tests {
			icon := ":white_check_mark:"
			if !t.Passed {
				icon = ":x:"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", t.Name, icon, formatGhaDuration(t.Duration))
		}
	}

	var failed []TestResultData
	var allowed []TestResultData
	for _, t := range g.tests {
		if !t.Passed && !t.AllowedFailure {
			failed = append(failed, t)
		}
		if t.AllowedFailure && !t.Passed {
			allowed = append(allowed, t)
		}
	}

	if len(failed) > 0 {
		fmt.Fprintf(&b, "\n### Failed Tests\n")
		fmt.Fprintf(&b, "| Test | Assertion | Error |\n")
		fmt.Fprintf(&b, "|------|-----------|-------|\n")
		for _, t := range failed {
			for _, a := range t.Assertions {
				if !a.Passed {
					fmt.Fprintf(&b, "| %s | %s | expected %s, got %s |\n", t.Name, a.Type, a.Expected, a.Actual)
				}
			}
		}
	}

	if len(allowed) > 0 {
		fmt.Fprintf(&b, "\n### Allowed Failures\n")
		for _, t := range allowed {
			fmt.Fprintf(&b, "- %s (allowed failure)\n", t.Name)
		}
	}

	return b.String()
}

func (g *GitHubActions) writeSummary(md string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		fmt.Fprint(g.w, md)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprint(g.w, md)
		return
	}
	defer f.Close()
	fmt.Fprint(f, md)
}

func (g *GitHubActions) emitCommands(_ SuiteResultData) {
	for _, t := range g.tests {
		if !t.Passed && !t.AllowedFailure {
			for _, a := range t.Assertions {
				if !a.Passed {
					msg := fmt.Sprintf("%s: %s expected %s, got %s", t.Name, a.Type, a.Expected, a.Actual)
					fmt.Fprintf(g.w, "::error title=Smoke Test Failed::%s\n", msg)
				}
			}
			if len(t.Assertions) == 0 {
				fmt.Fprintf(g.w, "::error title=Smoke Test Failed::%s\n", t.Name)
			}
		}
		if t.AllowedFailure && !t.Passed {
			fmt.Fprintf(g.w, "::warning title=Flaky Test::%s: allowed failure\n", t.Name)
		}
	}
}

func statusIcon(passed bool) string {
	if passed {
		return ":white_check_mark:"
	}
	return ":x:"
}

func formatGhaDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "0ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

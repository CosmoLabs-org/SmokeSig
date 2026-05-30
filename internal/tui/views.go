//go:build tui

package tui

import (
	"fmt"
	"strings"

	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
)

// applyFilter returns tests matching the given filter.
func applyFilter(tests []reporter.TestResultData, f Filter) []reporter.TestResultData {
	if f == FilterAll {
		return tests
	}
	var out []reporter.TestResultData
	for _, t := range tests {
		switch f {
		case FilterFailed:
			if !t.Passed && !t.Skipped {
				out = append(out, t)
			}
		case FilterPassed:
			if t.Passed {
				out = append(out, t)
			}
		case FilterSkipped:
			if t.Skipped {
				out = append(out, t)
			}
		}
	}
	return out
}

func viewHeader(m Model) string {
	var status string
	switch m.state {
	case StateRunning:
		status = styleDim.Render("RUNNING...")
	case StateRerunning:
		status = styleDim.Render("RE-RUNNING...")
	case StateResults:
		if m.results.Failed > 0 {
			status = styleFail.Render("FAIL")
		} else {
			status = stylePass.Render("PASS")
		}
	}

	project := m.project
	if project == "" {
		project = "SmokeSig"
	}

	filterLabel := ""
	switch m.filter {
	case FilterFailed:
		filterLabel = styleFail.Render(" [failed]")
	case FilterPassed:
		filterLabel = stylePass.Render(" [passed]")
	case FilterSkipped:
		filterLabel = styleSkip.Render(" [skipped]")
	}

	title := styleHeader.Render(fmt.Sprintf("%s  %s  %d tests%s",
		project, status, len(m.tests), filterLabel))
	return title
}

func viewTestList(m Model, maxHeight int) string {
	if len(m.tests) == 0 {
		if m.state == StateRunning || m.state == StateRerunning {
			return styleDim.Render("  Waiting for results...")
		}
		return styleDim.Render("  No tests match filter")
	}

	// Build lines with cursor and expansion
	var lines []string
	for i, t := range m.tests {
		line := renderTestLine(m, i, t)
		lines = append(lines, line)

		// If expanded, add detail lines
		isExpanded := m.allExpanded || m.expanded[i]
		if isExpanded {
			detail := viewDetail(t)
			if detail != "" {
				lines = append(lines, detail)
			}
		}
	}

	// Compute scroll window
	totalLines := len(lines)
	if totalLines <= maxHeight {
		return strings.Join(lines, "\n")
	}

	// Find the line index where the cursor's test starts
	cursorLine := 0
	for i := 0; i < m.cursor && i < len(m.tests); i++ {
		cursorLine++ // the test line itself
		isExpanded := m.allExpanded || m.expanded[i]
		if isExpanded {
			detail := viewDetail(m.tests[i])
			if detail != "" {
				cursorLine += countLines(detail)
			}
		}
	}

	// Scroll so cursor is visible
	scrollOffset := 0
	if cursorLine >= maxHeight {
		scrollOffset = cursorLine - maxHeight/2
	}
	if scrollOffset > totalLines-maxHeight {
		scrollOffset = totalLines - maxHeight
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	end := scrollOffset + maxHeight
	if end > totalLines {
		end = totalLines
	}

	return strings.Join(lines[scrollOffset:end], "\n")
}

func renderTestLine(m Model, idx int, t reporter.TestResultData) string {
	// Cursor indicator
	cursor := "  "
	if idx == m.cursor && m.state == StateResults {
		cursor = styleCursor.Render("> ")
	}

	// Status icon
	icon := statusIcon(t)

	// Duration
	dur := styleDim.Render(fmt.Sprintf("(%s)", t.Duration.Round(1e6)))

	// Name (dim during rerunning)
	name := t.Name
	if m.state == StateRerunning {
		name = styleDim.Render(name)
	}

	// Expand indicator
	isExpanded := m.allExpanded || m.expanded[idx]
	expandInd := ""
	if len(t.Assertions) > 0 || t.Error != nil {
		if isExpanded {
			expandInd = " " + styleDim.Render("[-]")
		} else {
			expandInd = " " + styleDim.Render("[+]")
		}
	}

	return fmt.Sprintf("%s%s %s %s%s", cursor, icon, name, dur, expandInd)
}

func statusIcon(t reporter.TestResultData) string {
	if t.Skipped {
		return styleSkip.Render("⊘") // circled dash
	}
	if t.AllowedFailure {
		return styleAllowed.Render("~")
	}
	if t.Passed {
		return stylePass.Render("✓") // checkmark
	}
	return styleFail.Render("✗") // cross
}

func viewDetail(t reporter.TestResultData) string {
	var lines []string

	for _, a := range t.Assertions {
		prefix := "    "
		if a.Passed {
			lines = append(lines, prefix+stylePass.Render("✓ ")+
				styleDim.Render(a.Type))
		} else {
			lines = append(lines, prefix+styleFail.Render("✗ ")+a.Type)
			if a.Expected != "" {
				lines = append(lines, prefix+"  expected: "+styleDim.Render(a.Expected))
			}
			if a.Actual != "" {
				lines = append(lines, prefix+"    actual: "+styleFail.Render(a.Actual))
			}
		}
	}

	if t.Error != nil {
		lines = append(lines, "    "+styleFail.Render("error: "+t.Error.Error()))
	}

	return strings.Join(lines, "\n")
}

func viewSummary(m Model) string {
	if m.state == StateRunning || m.state == StateRerunning {
		count := len(m.pending)
		return styleSummary.Render(fmt.Sprintf("%d tests completed...", count))
	}

	r := m.results
	parts := []string{
		fmt.Sprintf("%d tests", r.Total),
		stylePass.Render(fmt.Sprintf("%d passed", r.Passed)),
	}
	if r.Failed > 0 {
		parts = append(parts, styleFail.Render(fmt.Sprintf("%d failed", r.Failed)))
	}
	if r.Skipped > 0 {
		parts = append(parts, styleSkip.Render(fmt.Sprintf("%d skipped", r.Skipped)))
	}
	if r.AllowedFailures > 0 {
		parts = append(parts, styleAllowed.Render(fmt.Sprintf("%d allowed", r.AllowedFailures)))
	}
	parts = append(parts, styleDim.Render(fmt.Sprintf("(%s)", r.Duration.Round(1e6))))

	return styleSummary.Render(strings.Join(parts, "  "))
}

func viewShortcuts(m Model) string {
	if m.state == StateRunning || m.state == StateRerunning {
		return styleHelp.Render("[q]uit  [?]help")
	}
	return styleHelp.Render("[f]ilter  [r]erun  [R]erun-fails  [a]ll  [q]uit  [?]help")
}

func viewHelpFull(m Model) string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  SmokeSig Interactive TUI"))
	b.WriteString("\n\n")

	keys := m.keys
	helpItems := []struct {
		key  string
		desc string
	}{
		{keys.Up.Help().Key, keys.Up.Help().Desc},
		{keys.Down.Help().Key, keys.Down.Help().Desc},
		{keys.Top.Help().Key, keys.Top.Help().Desc},
		{keys.Bottom.Help().Key, keys.Bottom.Help().Desc},
		{keys.Toggle.Help().Key, keys.Toggle.Help().Desc},
		{keys.ToggleAll.Help().Key, keys.ToggleAll.Help().Desc},
		{keys.Filter.Help().Key, keys.Filter.Help().Desc},
		{keys.Rerun.Help().Key, keys.Rerun.Help().Desc},
		{keys.RerunFails.Help().Key, keys.RerunFails.Help().Desc},
		{keys.RerunAll.Help().Key, keys.RerunAll.Help().Desc},
		{keys.Help.Help().Key, keys.Help.Help().Desc},
		{keys.Quit.Help().Key, keys.Quit.Help().Desc},
	}

	for _, item := range helpItems {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", item.key, item.desc))
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  Press any key to close help"))
	return b.String()
}

func viewError(m Model) string {
	header := viewHeader(m)
	errMsg := styleError.Render(fmt.Sprintf("\n  Error: %v", m.runError))
	hint := styleDim.Render("\n\n  Press any key to dismiss")
	return header + errMsg + hint
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

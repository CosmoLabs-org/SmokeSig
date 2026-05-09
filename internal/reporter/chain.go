package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type formatEntry struct {
	filename string
	factory  func(io.Writer) Reporter
}

var formats = map[string]formatEntry{
	"terminal":   {"smoke-output.txt", func(w io.Writer) Reporter { return NewTerminal(w) }},
	"json":       {"smoke-results.json", func(w io.Writer) Reporter { return NewJSON(w) }},
	"junit":      {"smoke-junit.xml", func(w io.Writer) Reporter { return NewJUnit(w) }},
	"tap":        {"smoke-tap.txt", func(w io.Writer) Reporter { return NewTAP(w) }},
	"prometheus": {"smoke-metrics.prom", func(w io.Writer) Reporter { return NewPrometheus(w) }},
	"gha":       {"", func(w io.Writer) Reporter { return NewGitHubActions(w) }},
	"backstage": {"smoke-backstage.json", func(w io.Writer) Reporter { return NewBackstage(w) }},
}

// Chain parses a comma-separated format string, creates reporters for each
// format, and wraps them in a MultiReporter. The first format writes to
// stdout; subsequent formats write to auto-named files in the working
// directory. Returns the reporter, any opened files (creation order, close
// in reverse), or an error.
func Chain(format string, stdout io.Writer) (Reporter, []io.Closer, error) {
	names := parseFormats(format)
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no output format specified")
	}

	for _, n := range names {
		if _, ok := formats[n]; !ok {
			return nil, nil, fmt.Errorf("unknown format %q (valid: terminal, json, junit, tap, prometheus, gha, backstage)", n)
		}
	}

	var reporters []Reporter
	var closers []io.Closer

	for i, name := range names {
		entry := formats[name]
		var w io.Writer
		if i == 0 {
			w = stdout
		} else {
			f, err := os.Create(entry.filename)
			if err != nil {
				for j := len(closers) - 1; j >= 0; j-- {
					closers[j].Close()
				}
				return nil, nil, fmt.Errorf("creating %s: %w", entry.filename, err)
			}
			closers = append(closers, f)
			w = f
		}
		reporters = append(reporters, entry.factory(w))
	}

	if len(reporters) == 1 {
		return reporters[0], closers, nil
	}
	return NewMultiReporter(reporters...), closers, nil
}

func parseFormats(format string) []string {
	parts := strings.Split(format, ",")
	var names []string
	seen := make(map[string]bool)
	for _, p := range parts {
		name := strings.ToLower(strings.TrimSpace(p))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}


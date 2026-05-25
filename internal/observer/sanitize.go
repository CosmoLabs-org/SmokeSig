package observer

import (
	"regexp"
	"strings"
)

var (
	ansiRe      = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	timestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?`)
	uuidRe      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	durationRe  = regexp.MustCompile(`\d+(\.\d+)?(ms|s|m|h)`)
)

func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func StripTimestamps(s string) string {
	return timestampRe.ReplaceAllString(s, "<TIMESTAMP>")
}

func StripUUIDs(s string) string {
	return uuidRe.ReplaceAllString(s, "<UUID>")
}

func StripDurations(s string) string {
	return durationRe.ReplaceAllString(s, "<DURATION>")
}

func Sanitize(s string) string {
	s = StripANSI(s)
	s = StripTimestamps(s)
	s = StripUUIDs(s)
	s = StripDurations(s)
	return s
}

func ExtractKeyPhrases(s string) []string {
	lines := strings.Split(s, "\n")
	var nonBlank []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonBlank = append(nonBlank, trimmed)
		}
	}
	if len(nonBlank) == 0 {
		return nil
	}

	keywords := []string{"ready", "listening", "started", "complete", "done", "error", "fail", "running", "connected", "serving"}

	var matched []string
	for _, line := range nonBlank {
		lower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matched = append(matched, line)
				break
			}
		}
		if len(matched) >= 5 {
			return matched[:5]
		}
	}

	if len(matched) > 0 {
		if len(matched) > 5 {
			return matched[:5]
		}
		return matched
	}

	// No keywords matched — return first and last non-blank lines
	if len(nonBlank) == 1 {
		return []string{nonBlank[0]}
	}
	result := []string{nonBlank[0], nonBlank[len(nonBlank)-1]}
	if len(result) > 5 {
		return result[:5]
	}
	return result
}

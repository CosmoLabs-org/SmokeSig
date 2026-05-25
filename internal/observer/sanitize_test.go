package observer

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	input := "\x1b[31mred\x1b[0m normal \x1b[1mbold\x1b[0m"
	got := StripANSI(input)
	want := "red normal bold"
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestStripANSI_Empty(t *testing.T) {
	if got := StripANSI(""); got != "" {
		t.Errorf("StripANSI(\"\") = %q, want \"\"", got)
	}
}

func TestStripANSI_NoCodes(t *testing.T) {
	input := "plain text"
	if got := StripANSI(input); got != input {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, input)
	}
}

func TestStripTimestamps(t *testing.T) {
	input := "2024-01-15T10:30:00Z started"
	got := StripTimestamps(input)
	want := "<TIMESTAMP> started"
	if got != want {
		t.Errorf("StripTimestamps(%q) = %q, want %q", input, got, want)
	}
}

func TestStripTimestamps_WithOffset(t *testing.T) {
	input := "2024-01-15 10:30:00.123+03:00 event"
	got := StripTimestamps(input)
	want := "<TIMESTAMP> event"
	if got != want {
		t.Errorf("StripTimestamps(%q) = %q, want %q", input, got, want)
	}
}

func TestStripTimestamps_NoTimestamp(t *testing.T) {
	input := "no timestamp here"
	if got := StripTimestamps(input); got != input {
		t.Errorf("StripTimestamps(%q) = %q, want %q", input, got, input)
	}
}

func TestStripUUIDs(t *testing.T) {
	input := "id=550e8400-e29b-41d4-a716-446655440000"
	got := StripUUIDs(input)
	want := "id=<UUID>"
	if got != want {
		t.Errorf("StripUUIDs(%q) = %q, want %q", input, got, want)
	}
}

func TestStripUUIDs_Multiple(t *testing.T) {
	input := "a=550e8400-e29b-41d4-a716-446655440000 b=6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	got := StripUUIDs(input)
	want := "a=<UUID> b=<UUID>"
	if got != want {
		t.Errorf("StripUUIDs(%q) = %q, want %q", input, got, want)
	}
}

func TestStripDurations(t *testing.T) {
	input := "took 1.23s"
	got := StripDurations(input)
	want := "took <DURATION>"
	if got != want {
		t.Errorf("StripDurations(%q) = %q, want %q", input, got, want)
	}
}

func TestStripDurations_VariousUnits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"100ms elapsed", "<DURATION> elapsed"},
		{"waited 5m", "waited <DURATION>"},
		{"ran 2h", "ran <DURATION>"},
		{"3s timeout", "<DURATION> timeout"},
	}
	for _, tt := range tests {
		got := StripDurations(tt.input)
		if got != tt.want {
			t.Errorf("StripDurations(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	input := "\x1b[32m2024-01-15T10:30:00Z id=550e8400-e29b-41d4-a716-446655440000 took 1.23s\x1b[0m"
	got := Sanitize(input)
	if strings.Contains(got, "\x1b[") {
		t.Error("Sanitize() still contains ANSI codes")
	}
	if strings.Contains(got, "2024-01-15") {
		t.Error("Sanitize() still contains timestamp")
	}
	if strings.Contains(got, "550e8400") {
		t.Error("Sanitize() still contains UUID")
	}
	if strings.Contains(got, "1.23s") {
		t.Error("Sanitize() still contains duration")
	}
	want := "<TIMESTAMP> id=<UUID> took <DURATION>"
	if got != want {
		t.Errorf("Sanitize() = %q, want %q", got, want)
	}
}

func TestExtractKeyPhrases_WithKeywords(t *testing.T) {
	input := "line one\nServer started on port 8080\nsome noise\nDatabase connected successfully\nBuild complete\nerror in module X\nrunning tests"
	got := ExtractKeyPhrases(input)
	if len(got) > 5 {
		t.Errorf("ExtractKeyPhrases() returned %d results, want at most 5", len(got))
	}
	// Should contain keyword-matching lines
	for _, phrase := range got {
		lower := strings.ToLower(phrase)
		found := false
		for _, kw := range []string{"ready", "listening", "started", "complete", "done", "error", "fail", "running", "connected", "serving"} {
			if strings.Contains(lower, kw) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ExtractKeyPhrases() returned non-keyword line: %q", phrase)
		}
	}
}

func TestExtractKeyPhrases_CapAt5(t *testing.T) {
	lines := []string{
		"error one",
		"error two",
		"error three",
		"started something",
		"done with that",
		"running more",
		"complete final",
	}
	input := strings.Join(lines, "\n")
	got := ExtractKeyPhrases(input)
	if len(got) != 5 {
		t.Errorf("ExtractKeyPhrases() returned %d results, want 5", len(got))
	}
}

func TestExtractKeyPhrases_NoKeywords(t *testing.T) {
	input := "first line\nmiddle stuff\nlast line"
	got := ExtractKeyPhrases(input)
	if len(got) != 2 {
		t.Fatalf("ExtractKeyPhrases() returned %d results, want 2", len(got))
	}
	if got[0] != "first line" {
		t.Errorf("first result = %q, want %q", got[0], "first line")
	}
	if got[1] != "last line" {
		t.Errorf("last result = %q, want %q", got[1], "last line")
	}
}

func TestExtractKeyPhrases_Empty(t *testing.T) {
	got := ExtractKeyPhrases("")
	if got != nil {
		t.Errorf("ExtractKeyPhrases(\"\") = %v, want nil", got)
	}
}

func TestExtractKeyPhrases_BlankLines(t *testing.T) {
	got := ExtractKeyPhrases("  \n\n\t\n")
	if got != nil {
		t.Errorf("ExtractKeyPhrases(blank) = %v, want nil", got)
	}
}

func TestExtractKeyPhrases_SingleLine(t *testing.T) {
	got := ExtractKeyPhrases("just one line")
	if len(got) != 1 || got[0] != "just one line" {
		t.Errorf("ExtractKeyPhrases(single line) = %v, want [\"just one line\"]", got)
	}
}

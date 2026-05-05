package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

const shortTimeout = 5 * time.Second

func prereq(name, check, hint string) schema.Prerequisite {
	return schema.Prerequisite{Name: name, Check: check, Hint: hint}
}

// TestCheckPrerequisites_Empty verifies an empty list returns an empty slice.
func TestCheckPrerequisites_Empty(t *testing.T) {
	results := CheckPrerequisites(nil, shortTimeout)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}

	results = CheckPrerequisites([]schema.Prerequisite{}, shortTimeout)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty slice, got %d", len(results))
	}
}

// TestCheckPrerequisites_Passing verifies a command that exits 0 is marked Passed.
func TestCheckPrerequisites_Passing(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("echo-test", "echo hello", ""),
	}, shortTimeout)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Passed {
		t.Errorf("expected Passed=true, got false (error: %v)", r.Error)
	}
	if r.Error != nil {
		t.Errorf("expected nil Error, got %v", r.Error)
	}
}

// TestCheckPrerequisites_Failing verifies a command that exits non-zero is marked failed.
func TestCheckPrerequisites_Failing(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("false-test", "exit 1", "install the thing"),
	}, shortTimeout)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Passed {
		t.Error("expected Passed=false for exit 1, got true")
	}
	if r.Error == nil {
		t.Error("expected non-nil Error for exit 1, got nil")
	}
}

// TestCheckPrerequisites_OutputCapture verifies the first line of stdout is captured.
func TestCheckPrerequisites_OutputCapture(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("multi-line", "printf 'first-line\nsecond-line\n'", ""),
	}, shortTimeout)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Passed {
		t.Errorf("expected Passed=true, got false (error: %v)", r.Error)
	}
	if r.Output != "first-line" {
		t.Errorf("expected Output=%q, got %q", "first-line", r.Output)
	}
}

// TestCheckPrerequisites_OutputTrimmed verifies whitespace is trimmed from output.
func TestCheckPrerequisites_OutputTrimmed(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("echo-spaces", "echo '  trimmed  '", ""),
	}, shortTimeout)

	r := results[0]
	if strings.HasPrefix(r.Output, " ") || strings.HasSuffix(r.Output, " ") {
		t.Errorf("expected trimmed output, got %q", r.Output)
	}
	if r.Output != "trimmed" {
		t.Errorf("expected %q, got %q", "trimmed", r.Output)
	}
}

// TestCheckPrerequisites_NoOutput verifies a passing command with no stdout has empty Output.
func TestCheckPrerequisites_NoOutput(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("silent", "true", ""),
	}, shortTimeout)

	r := results[0]
	if !r.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	if r.Output != "" {
		t.Errorf("expected empty Output, got %q", r.Output)
	}
}

// TestCheckPrerequisites_Timeout verifies hung commands are killed after the timeout.
func TestCheckPrerequisites_Timeout(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("slow-cmd", "sleep 10", "check your setup"),
	}, 100*time.Millisecond)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Passed {
		t.Error("expected Passed=false for timed-out command, got true")
	}
	if r.Error == nil {
		t.Error("expected non-nil Error for timed-out command, got nil")
	}
}

// TestCheckPrerequisites_HintPassthrough verifies the Hint field is copied to the result.
func TestCheckPrerequisites_HintPassthrough(t *testing.T) {
	hint := "run: brew install foo"
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("hinted", "true", hint),
	}, shortTimeout)

	r := results[0]
	if r.Hint != hint {
		t.Errorf("expected Hint=%q, got %q", hint, r.Hint)
	}
}

// TestCheckPrerequisites_HintOnFailure verifies hint is available even when check fails.
func TestCheckPrerequisites_HintOnFailure(t *testing.T) {
	hint := "install missing tool"
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("missing-tool", "exit 127", hint),
	}, shortTimeout)

	r := results[0]
	if r.Passed {
		t.Error("expected Passed=false")
	}
	if r.Hint != hint {
		t.Errorf("expected Hint=%q, got %q", hint, r.Hint)
	}
}

// TestCheckPrerequisites_NamePreserved verifies the Name field is copied to the result.
func TestCheckPrerequisites_NamePreserved(t *testing.T) {
	results := CheckPrerequisites([]schema.Prerequisite{
		prereq("my-prereq", "true", ""),
	}, shortTimeout)

	r := results[0]
	if r.Name != "my-prereq" {
		t.Errorf("expected Name=%q, got %q", "my-prereq", r.Name)
	}
}

// TestCheckPrerequisites_AllRun verifies all prerequisites run even when one fails.
func TestCheckPrerequisites_AllRun(t *testing.T) {
	prereqs := []schema.Prerequisite{
		prereq("pass-1", "echo ok", ""),
		prereq("fail-1", "exit 1", ""),
		prereq("pass-2", "echo ok", ""),
	}
	results := CheckPrerequisites(prereqs, shortTimeout)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d — all prereqs must run", len(results))
	}
	if !results[0].Passed {
		t.Error("results[0] (pass-1) should be Passed=true")
	}
	if results[1].Passed {
		t.Error("results[1] (fail-1) should be Passed=false")
	}
	if !results[2].Passed {
		t.Error("results[2] (pass-2) should be Passed=true")
	}
}

// TestCheckPrerequisites_OrderPreserved verifies results order matches input order.
func TestCheckPrerequisites_OrderPreserved(t *testing.T) {
	prereqs := []schema.Prerequisite{
		prereq("alpha", "echo alpha", ""),
		prereq("beta", "echo beta", ""),
		prereq("gamma", "echo gamma", ""),
	}
	results := CheckPrerequisites(prereqs, shortTimeout)

	for i, p := range prereqs {
		if results[i].Name != p.Name {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, p.Name)
		}
	}
}

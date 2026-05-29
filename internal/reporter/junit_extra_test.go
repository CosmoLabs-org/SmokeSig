package reporter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestJUnit_OutputIsValidXML(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:     "build",
		Passed:   true,
		Duration: 100 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
		},
	})

	r.Summary(SuiteResultData{
		Project:  "valid-xml",
		Total:    1,
		Passed:   1,
		Duration: 100 * time.Millisecond,
	})

	// Strip XML declaration for strict parsing.
	raw := buf.String()
	idx := strings.Index(raw, "\n")
	if idx >= 0 {
		raw = raw[idx+1:]
	}

	var doc struct {
		XMLName xml.Name `xml:"testsuites"`
	}
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, buf.String())
	}
	if doc.XMLName.Local != "testsuites" {
		t.Errorf("root element = %q, want %q", doc.XMLName.Local, "testsuites")
	}
}

func TestJUnit_TestSuiteCorrectTestCount(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{Name: "t1", Passed: true, Duration: 50 * time.Millisecond})
	r.TestResult(TestResultData{Name: "t2", Passed: false, Duration: 30 * time.Millisecond,
		Assertions: []AssertionDetail{{Type: "exit_code", Expected: "0", Actual: "1", Passed: false}}})
	r.TestResult(TestResultData{Name: "t3", Skipped: true, Duration: 0})
	r.TestResult(TestResultData{Name: "t4", Passed: true, Duration: 20 * time.Millisecond})

	r.Summary(SuiteResultData{
		Project:  "count-test",
		Total:    4,
		Passed:   2,
		Failed:   1,
		Skipped:  1,
		Duration: 100 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)

	if result.Tests != 4 {
		t.Errorf("testsuites tests = %d, want 4", result.Tests)
	}
	if result.Failures != 1 {
		t.Errorf("testsuites failures = %d, want 1", result.Failures)
	}

	suite := result.Suites[0]
	if suite.Tests != 4 {
		t.Errorf("testsuite tests = %d, want 4", suite.Tests)
	}
	if suite.Failures != 1 {
		t.Errorf("testsuite failures = %d, want 1", suite.Failures)
	}
	if suite.Skipped != 1 {
		t.Errorf("testsuite skipped = %d, want 1", suite.Skipped)
	}
	if len(suite.Cases) != 4 {
		t.Fatalf("testcase count = %d, want 4", len(suite.Cases))
	}
}

func TestJUnit_TestCaseHasNameAndClassname(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:     "deploy-check",
		Passed:   true,
		Duration: 200 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "http", Expected: "200", Actual: "200", Passed: true},
		},
	})

	r.Summary(SuiteResultData{
		Project:  "name-test",
		Total:    1,
		Passed:   1,
		Duration: 200 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)
	suite := result.Suites[0]

	if len(suite.Cases) != 1 {
		t.Fatalf("testcase count = %d, want 1", len(suite.Cases))
	}

	tc := suite.Cases[0]
	if tc.Name != "deploy-check" {
		t.Errorf("testcase name = %q, want %q", tc.Name, "deploy-check")
	}

	// Verify the raw XML contains the name attribute.
	raw := buf.String()
	if !strings.Contains(raw, `name="deploy-check"`) {
		t.Errorf("raw XML should contain name attribute: %s", raw)
	}
}

func TestJUnit_FailedTestHasFailureWithMessage(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:   "health-endpoint",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "http", Expected: "200", Actual: "503", Passed: false},
			{Type: "stdout_contains", Expected: "healthy", Actual: "unhealthy", Passed: false},
		},
		Duration: 150 * time.Millisecond,
		Error:    fmt.Errorf("connection refused"),
	})

	r.Summary(SuiteResultData{
		Project:  "fail-test",
		Total:    1,
		Failed:   1,
		Duration: 150 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)
	tc := result.Suites[0].Cases[0]

	if tc.Failure == nil {
		t.Fatal("failed test must have a <failure> element")
	}

	// Failure message should contain assertion type info.
	msg := tc.Failure.Message
	if msg == "" {
		t.Error("failure message should not be empty")
	}
	if !strings.Contains(msg, "http") {
		t.Errorf("failure message should mention assertion type 'http': %q", msg)
	}
	if !strings.Contains(msg, "stdout_contains") {
		t.Errorf("failure message should mention assertion type 'stdout_contains': %q", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("failure message should contain error text: %q", msg)
	}

	// Failure body should have detailed expected/actual info.
	if !strings.Contains(tc.Failure.Text, "Expected") {
		t.Errorf("failure body should contain 'Expected': %q", tc.Failure.Text)
	}
}

func TestJUnit_SkippedTestHasSkippedElement(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:     "optional-integration",
		Skipped:  true,
		Duration: 0,
	})

	r.Summary(SuiteResultData{
		Project:  "skip-test",
		Total:    1,
		Skipped:  1,
		Duration: 0,
	})

	result := parseJUnit(t, &buf)
	tc := result.Suites[0].Cases[0]

	if tc.Skipped == nil {
		t.Error("skipped test must have a <skipped> element")
	}
	if tc.Failure != nil {
		t.Error("skipped test should not have a <failure> element")
	}

	// Verify raw XML contains the skipped element.
	raw := buf.String()
	if !strings.Contains(raw, "<skipped") {
		t.Error("raw XML should contain <skipped> element")
	}
}

func TestJUnit_AllowedFailureTreatedAsSkipped(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:           "flaky-test",
		Passed:         false,
		AllowedFailure: true,
		Duration:       50 * time.Millisecond,
	})

	r.Summary(SuiteResultData{
		Project:         "allowed-test",
		Total:           1,
		AllowedFailures: 1,
		Duration:        50 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)
	tc := result.Suites[0].Cases[0]

	if tc.Skipped == nil {
		t.Error("allowed-failure test should have a <skipped> element")
	}
	if tc.Failure != nil {
		t.Error("allowed-failure test should not have a <failure> element")
	}
}

func TestJUnit_PropertiesIncludeHostnameAndTimestamp(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	r.TestResult(TestResultData{
		Name:     "prop-check",
		Passed:   true,
		Duration: 10 * time.Millisecond,
	})

	r.Summary(SuiteResultData{
		Project:  "props-test",
		Total:    1,
		Passed:   1,
		Duration: 10 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)
	suite := result.Suites[0]

	// Verify timestamp is set and parseable as RFC3339.
	if suite.Timestamp == "" {
		t.Error("testsuite should have a timestamp attribute")
	} else {
		_, err := time.Parse(time.RFC3339, suite.Timestamp)
		if err != nil {
			t.Errorf("timestamp %q is not valid RFC3339: %v", suite.Timestamp, err)
		}
	}

	// Verify hostname is set.
	if suite.Hostname == "" {
		t.Error("testsuite should have a hostname attribute")
	}

	// Verify properties contain project metadata.
	if suite.Properties == nil {
		t.Fatal("testsuite should have a <properties> element")
	}

	propMap := make(map[string]string)
	for _, p := range suite.Properties.Props {
		propMap[p.Name] = p.Value
	}

	for _, key := range []string{"project", "passed", "failed", "skipped"} {
		if _, ok := propMap[key]; !ok {
			t.Errorf("properties should contain key %q", key)
		}
	}

	if propMap["project"] != "props-test" {
		t.Errorf("project property = %q, want %q", propMap["project"], "props-test")
	}
}

func TestJUnit_FailedTestNoAssertionsNoError(t *testing.T) {
	var buf bytes.Buffer
	r := NewJUnit(&buf)

	// Failed test with no assertions and no error — triggers the `message = "test failed"` branch
	r.TestResult(TestResultData{
		Name:       "bare-fail",
		Passed:     false,
		Duration:   10 * time.Millisecond,
		Assertions: nil,
		Error:      nil,
	})

	r.Summary(SuiteResultData{
		Project:  "bare-fail-suite",
		Total:    1,
		Failed:   1,
		Duration: 10 * time.Millisecond,
	})

	result := parseJUnit(t, &buf)
	tc := result.Suites[0].Cases[0]

	if tc.Failure == nil {
		t.Fatal("expected failure element for bare failed test")
	}
	if tc.Failure.Message != "test failed" {
		t.Errorf("failure message = %q, want 'test failed'", tc.Failure.Message)
	}
}

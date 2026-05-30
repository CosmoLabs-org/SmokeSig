package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestJSON_ValidOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSON(&buf)

	r.PrereqStart("Go installed")
	r.PrereqResult(PrereqResultData{Name: "Go installed", Passed: true, Output: "go1.26.2"})

	r.TestStart("Compiles")
	r.TestResult(TestResultData{
		Name:   "Compiles",
		Passed: true,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "0", Passed: true},
		},
		Duration: 100 * time.Millisecond,
	})

	r.TestStart("Fails")
	r.TestResult(TestResultData{
		Name:   "Fails",
		Passed: false,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
		Duration: 50 * time.Millisecond,
		Error:    fmt.Errorf("exit code 1"),
	})

	r.Summary(SuiteResultData{
		Project:  "myapp",
		Total:    2,
		Passed:   1,
		Failed:   1,
		Duration: 150 * time.Millisecond,
	})

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result["project"] != "myapp" {
		t.Errorf("project = %v", result["project"])
	}
	if result["total"].(float64) != 2 {
		t.Errorf("total = %v", result["total"])
	}

	prereqs, ok := result["prerequisites"].([]interface{})
	if !ok {
		t.Fatal("prerequisites not an array")
	}
	if len(prereqs) != 1 {
		t.Errorf("prereqs count = %d", len(prereqs))
	}
	p0 := prereqs[0].(map[string]interface{})
	if p0["output"] != "go1.26.2" {
		t.Errorf("prereq output = %v", p0["output"])
	}

	tests, ok := result["tests"].([]interface{})
	if !ok {
		t.Fatal("tests not an array")
	}
	if len(tests) != 2 {
		t.Errorf("tests count = %d", len(tests))
	}

	t1 := tests[1].(map[string]interface{})
	if t1["error"] != "exit code 1" {
		t.Errorf("test error = %v", t1["error"])
	}
}

func TestJSON_EmptyResults(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSON(&buf)
	r.Summary(SuiteResultData{Project: "empty", Total: 0})

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["total"].(float64) != 0 {
		t.Errorf("total = %v", result["total"])
	}
}

func TestJSON_PrereqHint(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSON(&buf)
	r.PrereqResult(PrereqResultData{Name: "Docker", Passed: false, Hint: "Install Docker"})
	r.Summary(SuiteResultData{Project: "test"})

	var result map[string]interface{}
	json.Unmarshal(buf.Bytes(), &result)
	prereqs := result["prerequisites"].([]interface{})
	p := prereqs[0].(map[string]interface{})
	if p["hint"] != "Install Docker" {
		t.Errorf("hint = %v", p["hint"])
	}
}

func TestJSON_PrereqWithError(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSON(&buf)
	r.PrereqResult(PrereqResultData{
		Name:   "Docker",
		Passed: false,
		Error:  fmt.Errorf("docker daemon not running"),
	})
	r.Summary(SuiteResultData{Project: "test"})

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	prereqs := result["prerequisites"].([]interface{})
	p := prereqs[0].(map[string]interface{})
	if p["error"] != "docker daemon not running" {
		t.Errorf("prereq error = %v, want 'docker daemon not running'", p["error"])
	}
}

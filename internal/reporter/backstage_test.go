package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBackstage_AllPassing(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "api-health", Passed: true, Duration: 120 * time.Millisecond})
	b.TestResult(TestResultData{Name: "db-conn", Passed: true, Duration: 50 * time.Millisecond})
	b.Summary(SuiteResultData{Project: "my-service", Total: 2, Passed: 2})

	var entity backstageEntity
	if err := json.Unmarshal(buf.Bytes(), &entity); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entity.Status.HealthCheck.Status != "healthy" {
		t.Errorf("expected healthy, got %s", entity.Status.HealthCheck.Status)
	}
	if len(entity.Status.HealthCheck.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(entity.Status.HealthCheck.Checks))
	}
	for _, c := range entity.Status.HealthCheck.Checks {
		if c.Status != "healthy" {
			t.Errorf("check %s: expected healthy, got %s", c.Name, c.Status)
		}
	}
}

func TestBackstage_OneFailure(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "api-health", Passed: true, Duration: 100 * time.Millisecond})
	b.TestResult(TestResultData{
		Name:     "db-conn",
		Passed:   false,
		Duration: 302 * time.Millisecond,
		Assertions: []AssertionDetail{
			{Type: "exit_code", Expected: "0", Actual: "1", Passed: false},
		},
	})
	b.Summary(SuiteResultData{Project: "my-service", Total: 2, Passed: 1, Failed: 1})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	if entity.Status.HealthCheck.Status != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", entity.Status.HealthCheck.Status)
	}
	failed := entity.Status.HealthCheck.Checks[1]
	if failed.Status != "unhealthy" {
		t.Errorf("expected unhealthy check, got %s", failed.Status)
	}
	if !strings.Contains(failed.Message, "exit_code") {
		t.Errorf("expected assertion error message, got %q", failed.Message)
	}
}

func TestBackstage_AllowedFailure(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "api", Passed: true, Duration: 50 * time.Millisecond})
	b.TestResult(TestResultData{
		Name:           "flaky-ext",
		Passed:         false,
		AllowedFailure: true,
		Duration:       200 * time.Millisecond,
	})
	b.Summary(SuiteResultData{Project: "svc", Total: 2, Passed: 1, Failed: 0, AllowedFailures: 1})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	if entity.Status.HealthCheck.Status != "degraded" {
		t.Errorf("expected degraded, got %s", entity.Status.HealthCheck.Status)
	}
}

func TestBackstage_EmptySuite(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.Summary(SuiteResultData{Project: "empty"})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	if entity.Status.HealthCheck.Status != "unknown" {
		t.Errorf("expected unknown, got %s", entity.Status.HealthCheck.Status)
	}
	if len(entity.Status.HealthCheck.Checks) != 0 {
		t.Errorf("expected 0 checks, got %d", len(entity.Status.HealthCheck.Checks))
	}
}

func TestBackstage_JSONValidity(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "test", Passed: true, Duration: 10 * time.Millisecond})
	b.Summary(SuiteResultData{Project: "svc", Total: 1, Passed: 1})

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if raw["apiVersion"] != "backstage.io/v1alpha1" {
		t.Errorf("wrong apiVersion: %v", raw["apiVersion"])
	}
	if raw["kind"] != "Component" {
		t.Errorf("wrong kind: %v", raw["kind"])
	}
}

func TestBackstage_Annotations(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{Name: "t", Passed: true, Duration: 5 * time.Millisecond})
	b.Summary(SuiteResultData{Project: "svc", Total: 3, Passed: 2, Failed: 1})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	ann := entity.Metadata.Annotations
	for _, key := range []string{
		"backstage.io/smoke-status",
		"backstage.io/smoke-passed",
		"backstage.io/smoke-failed",
		"backstage.io/smoke-total",
		"backstage.io/smoke-timestamp",
	} {
		if _, ok := ann[key]; !ok {
			t.Errorf("missing annotation %s", key)
		}
	}
	if ann["backstage.io/smoke-passed"] != "2" {
		t.Errorf("smoke-passed: got %q", ann["backstage.io/smoke-passed"])
	}
}

func TestBackstage_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.Summary(SuiteResultData{Project: "svc", Total: 0, Passed: 0})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	ts := entity.Metadata.Annotations["backstage.io/smoke-timestamp"]
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("timestamp %q is not RFC 3339: %v", ts, err)
	}
}

func TestBackstage_ErrorWithoutAssertions(t *testing.T) {
	var buf bytes.Buffer
	b := NewBackstage(&buf)
	b.TestResult(TestResultData{
		Name:     "exec-fail",
		Passed:   false,
		Duration: 10 * time.Millisecond,
		Error:    errors.New("command not found"),
	})
	b.Summary(SuiteResultData{Project: "svc", Total: 1, Passed: 0, Failed: 1})

	var entity backstageEntity
	json.Unmarshal(buf.Bytes(), &entity)

	check := entity.Status.HealthCheck.Checks[0]
	if !strings.Contains(check.Message, "command not found") {
		t.Errorf("expected error message in check, got %q", check.Message)
	}
}

package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Backstage struct {
	w     io.Writer
	tests []TestResultData
}

func NewBackstage(w io.Writer) *Backstage {
	return &Backstage{w: w}
}

func (b *Backstage) PrereqStart(_ string)            {}
func (b *Backstage) PrereqResult(_ PrereqResultData) {}
func (b *Backstage) TestStart(_ string)              {}

func (b *Backstage) TestResult(r TestResultData) {
	b.tests = append(b.tests, r)
}

func (b *Backstage) Summary(s SuiteResultData) {
	entity := buildBackstageEntity(s, b.tests)
	enc := json.NewEncoder(b.w)
	enc.SetIndent("", "  ")
	enc.Encode(entity)
}

func buildBackstageEntity(s SuiteResultData, tests []TestResultData) backstageEntity {
	status := backstageOverallStatus(tests)
	now := time.Now().UTC().Format(time.RFC3339)

	checks := make([]backstageCheck, 0, len(tests))
	for _, t := range tests {
		cs := "healthy"
		msg := ""
		if !t.Passed {
			cs = "unhealthy"
			for _, a := range t.Assertions {
				if !a.Passed {
					msg = fmt.Sprintf("%s: expected %s, got %s", a.Type, a.Expected, a.Actual)
					break
				}
			}
			if msg == "" && t.Error != nil {
				msg = t.Error.Error()
			}
		}
		if t.AllowedFailure && !t.Passed {
			cs = "degraded"
			if msg == "" {
				msg = "allowed failure"
			}
		}
		checks = append(checks, backstageCheck{
			Name:       t.Name,
			Status:     cs,
			DurationMs: t.Duration.Milliseconds(),
			Message:    msg,
		})
	}

	return backstageEntity{
		APIVersion: "backstage.io/v1alpha1",
		Kind:       "Component",
		Metadata: backstageMetadata{
			Name: s.Project,
			Annotations: map[string]string{
				"backstage.io/smoke-status":    status,
				"backstage.io/smoke-passed":    fmt.Sprintf("%d", s.Passed),
				"backstage.io/smoke-failed":    fmt.Sprintf("%d", s.Failed),
				"backstage.io/smoke-total":     fmt.Sprintf("%d", s.Total),
				"backstage.io/smoke-timestamp": now,
			},
		},
		Status: backstageStatus{
			HealthCheck: backstageHealthCheck{
				Status: status,
				Checks: checks,
			},
		},
	}
}

func backstageOverallStatus(tests []TestResultData) string {
	if len(tests) == 0 {
		return "unknown"
	}
	hasFailure := false
	hasAllowedFailure := false
	for _, t := range tests {
		if !t.Passed && !t.AllowedFailure {
			return "unhealthy"
		}
		if !t.Passed && t.AllowedFailure {
			hasAllowedFailure = true
		}
		if !t.Passed {
			hasFailure = true
		}
	}
	if hasAllowedFailure {
		return "degraded"
	}
	if hasFailure {
		return "unhealthy"
	}
	return "healthy"
}

type backstageEntity struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   backstageMetadata `json:"metadata"`
	Status     backstageStatus   `json:"status"`
}

type backstageMetadata struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

type backstageStatus struct {
	HealthCheck backstageHealthCheck `json:"healthcheck"`
}

type backstageHealthCheck struct {
	Status string           `json:"status"`
	Checks []backstageCheck `json:"checks"`
}

type backstageCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Message    string `json:"message,omitempty"`
}

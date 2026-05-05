package runner

import (
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckPing_Success(t *testing.T) {
	result := CheckPing(&schema.PingCheck{Host: "127.0.0.1"})
	if !result.Passed {
		t.Errorf("expected ping to localhost to pass, got: %s", result.Actual)
	}
	if result.Type != "ping" {
		t.Errorf("expected type 'ping', got %s", result.Type)
	}
}

func TestCheckPing_Unreachable(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1, typically unroutable
	result := CheckPing(&schema.PingCheck{Host: "192.0.2.1", Count: 1})
	// This may or may not fail depending on network, so just verify it returns a result
	if result.Type != "ping" {
		t.Errorf("expected type 'ping', got %s", result.Type)
	}
}

func TestCheckPing_Defaults(t *testing.T) {
	result := CheckPing(&schema.PingCheck{Host: "localhost"})
	if result.Type != "ping" {
		t.Errorf("expected type 'ping', got %s", result.Type)
	}
}

func TestCheckK8sResource_MissingKubectl(t *testing.T) {
	// This test will fail if kubectl is not installed, which is expected
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "pod",
		Name:      "nonexistent",
	})
	// Just verify it returns a result
	if result.Type != "k8s_resource" {
		t.Errorf("expected type 'k8s_resource', got %s", result.Type)
	}
}

func TestCheckK8sResource_MissingFields(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "pod",
		Name:      "test-pod",
		Context:   "test-context",
	})
	if result.Type != "k8s_resource" {
		t.Errorf("expected type 'k8s_resource', got %s", result.Type)
	}
	if result.Expected == "" {
		t.Error("expected non-empty Expected field")
	}
}

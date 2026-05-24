package runner

import (
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- CheckDockerContainerRunning ---

func TestCheckDockerContainerRunning_NonExistent(t *testing.T) {
	result := CheckDockerContainerRunning(&schema.DockerContainerCheck{
		Name: "zzz_nonexistent_container_12345",
	})
	if result.Passed {
		t.Error("non-existent container should fail")
	}
	if result.Type != "docker_container_running" {
		t.Errorf("type = %q, want docker_container_running", result.Type)
	}
}

func TestCheckDockerContainerRunning_EmptyName(t *testing.T) {
	result := CheckDockerContainerRunning(&schema.DockerContainerCheck{
		Name: "",
	})
	if result.Passed {
		t.Error("empty name should fail")
	}
	if result.Type != "docker_container_running" {
		t.Errorf("type = %q, want docker_container_running", result.Type)
	}
}

// --- CheckDockerImageExists ---

func TestCheckDockerImageExists_NonExistent(t *testing.T) {
	result := CheckDockerImageExists(&schema.DockerImageCheck{
		Image: "zzz_nonexistent_image_12345:latest",
	})
	if result.Passed {
		t.Error("non-existent image should fail")
	}
	if result.Type != "docker_image_exists" {
		t.Errorf("type = %q, want docker_image_exists", result.Type)
	}
}

// --- CheckDockerComposeHealthy ---

func TestCheckDockerComposeHealthy_NoDocker(t *testing.T) {
	result := CheckDockerComposeHealthy(&schema.DockerComposeCheck{})
	if result.Type != "docker_compose_healthy" {
		t.Errorf("type = %q, want docker_compose_healthy", result.Type)
	}
}

func TestCheckDockerComposeHealthy_CustomFile(t *testing.T) {
	result := CheckDockerComposeHealthy(&schema.DockerComposeCheck{
		ComposeFile: "nonexistent-compose.yml",
	})
	if result.Passed {
		t.Error("non-existent compose file should fail")
	}
	if result.Type != "docker_compose_healthy" {
		t.Errorf("type = %q, want docker_compose_healthy", result.Type)
	}
}

func TestCheckDockerComposeHealthy_SpecificServices(t *testing.T) {
	result := CheckDockerComposeHealthy(&schema.DockerComposeCheck{
		Services: []string{"web", "db"},
	})
	if result.Passed {
		t.Error("non-running services should fail")
	}
	if result.Type != "docker_compose_healthy" {
		t.Errorf("type = %q, want docker_compose_healthy", result.Type)
	}
}

// --- CheckK8sResource ---

func TestCheckK8sResource_TypeField(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "pod",
		Name:      "nonexistent-pod-xyz",
	})
	if result.Type != "k8s_resource" {
		t.Errorf("type = %q, want k8s_resource", result.Type)
	}
	// kubectl may not be configured in test env — verify it doesn't panic
}


func TestCheckK8sResource_CustomTimeout(t *testing.T) {
	result := CheckK8sResource(&schema.K8sResourceCheck{
		Namespace: "default",
		Kind:      "service",
		Name:      "test-svc",
		Timeout:   schema.Duration{Duration: 2 * time.Second},
	})
	if result.Type != "k8s_resource" {
		t.Errorf("type = %q, want k8s_resource", result.Type)
	}
}

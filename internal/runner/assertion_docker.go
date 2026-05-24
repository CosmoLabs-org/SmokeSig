package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// isDockerAvailable returns true if the docker daemon is reachable.
func isDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

// CheckDockerContainerRunning checks if a named Docker container is running.
func CheckDockerContainerRunning(check *schema.DockerContainerCheck) AssertionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format={{.State.Running}}", check.Name).Output()
	if err != nil {
		return AssertionResult{Type: "docker_container_running", Expected: check.Name, Actual: "container not found or docker unavailable: " + err.Error(), Passed: false}
	}
	running := strings.TrimSpace(string(out))
	if running != "true" {
		return AssertionResult{Type: "docker_container_running", Expected: "true", Actual: running, Passed: false}
	}
	return AssertionResult{Type: "docker_container_running", Expected: check.Name, Actual: "running", Passed: true}
}

// CheckDockerImageExists checks if a Docker image exists locally.
func CheckDockerImageExists(check *schema.DockerImageCheck) AssertionResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, "docker", "image", "inspect", check.Image).Run()
	if err != nil {
		return AssertionResult{Type: "docker_image_exists", Expected: check.Image, Actual: "image not found or docker unavailable: " + err.Error(), Passed: false}
	}
	return AssertionResult{Type: "docker_image_exists", Expected: check.Image, Actual: "exists", Passed: true}
}

// composeService represents a service row from docker compose ps --format json.
type composeService struct {
	Name   string `json:"Name"`
	State  string `json:"State"`
	Health string `json:"Health"`
}

// CheckDockerComposeHealthy checks that Docker Compose services are running and healthy.
func CheckDockerComposeHealthy(check *schema.DockerComposeCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"compose", "ps", "--format", "json"}
	if check.ComposeFile != "" {
		args = []string{"compose", "-f", check.ComposeFile, "ps", "--format", "json"}
	}

	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "compose services running",
			Actual:   fmt.Sprintf("docker compose ps failed: %v", err),
			Passed:   false,
		}
	}

	var services []composeService
	if err := json.Unmarshal(out, &services); err != nil {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "compose services running",
			Actual:   fmt.Sprintf("parse failed: %v", err),
			Passed:   false,
		}
	}

	if len(services) == 0 {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "compose services running",
			Actual:   "no services found",
			Passed:   false,
		}
	}

	serviceFilter := make(map[string]bool)
	for _, s := range check.Services {
		serviceFilter[s] = true
	}

	var unhealthy []string
	for _, svc := range services {
		if len(serviceFilter) > 0 && !serviceFilter[svc.Name] {
			continue
		}
		if svc.State != "running" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", svc.Name, svc.State))
		} else if svc.Health != "" && svc.Health != "healthy" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", svc.Name, svc.Health))
		}
	}

	if len(unhealthy) > 0 {
		return AssertionResult{
			Type:     "docker_compose_healthy",
			Expected: "all services healthy",
			Actual:   strings.Join(unhealthy, ", "),
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "docker_compose_healthy",
		Expected: "all services healthy",
		Actual:   fmt.Sprintf("%d services healthy", len(services)),
		Passed:   true,
	}
}

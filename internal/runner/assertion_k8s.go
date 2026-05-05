package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckK8sResource verifies a Kubernetes resource exists and optionally meets a condition.
func CheckK8sResource(check *schema.K8sResourceCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	args := []string{"get", check.Kind, check.Name, "-n", check.Namespace}
	if check.Context != "" {
		args = append(args, "--context", check.Context)
	}

	if check.Condition != "" {
		args = append(args, "-o", fmt.Sprintf("jsonpath={.status.conditions[?(@.type==\"%s\")].status}", check.Condition))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return AssertionResult{Type: "k8s_resource", Expected: fmt.Sprintf("%s/%s in %s", check.Kind, check.Name, check.Namespace), Actual: fmt.Sprintf("kubectl failed: %s: %s", err, strings.TrimSpace(string(out))), Passed: false}
	}

	if check.Condition != "" {
		status := strings.TrimSpace(string(out))
		if status != "True" {
			return AssertionResult{Type: "k8s_resource", Expected: fmt.Sprintf("%s=True", check.Condition), Actual: status, Passed: false}
		}
		return AssertionResult{Type: "k8s_resource", Expected: fmt.Sprintf("%s/%s in %s (%s=True)", check.Kind, check.Name, check.Namespace, check.Condition), Actual: "condition met", Passed: true}
	}

	return AssertionResult{Type: "k8s_resource", Expected: fmt.Sprintf("%s/%s in %s", check.Kind, check.Name, check.Namespace), Actual: "resource exists", Passed: true}
}

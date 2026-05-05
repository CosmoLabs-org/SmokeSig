package runner

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckPing verifies a host responds to ICMP echo requests via the system ping command.
func CheckPing(check *schema.PingCheck) AssertionResult {
	host := check.Host
	count := check.Count
	if count == 0 {
		count = 1
	}
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Overall command timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Duration(count)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", fmt.Sprintf("%d", count), "-w", fmt.Sprintf("%d", timeout.Milliseconds()), host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", fmt.Sprintf("%d", count), "-W", fmt.Sprintf("%d", int(timeout.Seconds())), host)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return AssertionResult{Type: "ping", Expected: host, Actual: fmt.Sprintf("ping failed: %s: %s", err, strings.TrimSpace(string(out))), Passed: false}
	}

	output := string(out)
	if strings.Contains(output, "100% packet loss") || strings.Contains(output, "(100% loss)") {
		return AssertionResult{Type: "ping", Expected: host, Actual: "100% packet loss", Passed: false}
	}

	return AssertionResult{Type: "ping", Expected: host, Actual: "host reachable", Passed: true}
}

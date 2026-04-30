package runner

import (
	"bufio"
	"context"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// RunLifecycleHooks executes a sequence of lifecycle hooks.
// Hooks run sequentially via "sh -c <command>".
// When EnvPass is true, lines matching KEY=VALUE are captured and merged into the returned env map.
// The first non-nil error is returned, but hooks with AlwaysRun=true execute even after errors.
// The configDir specifies the working directory for commands (empty = current directory).
func RunLifecycleHooks(ctx context.Context, hooks []schema.LifecycleHook, env map[string]string, configDir string) (map[string]string, error) {
	if env == nil {
		env = make(map[string]string)
	}

	var firstErr error
	envCopy := make(map[string]string, len(env))
	for k, v := range env {
		envCopy[k] = v
	}

	for i, hook := range hooks {
		if hook.Command == "" {
			continue
		}

		skipDueToError := firstErr != nil && !hook.AlwaysRun
		if skipDueToError {
			continue
		}

		timeout := hook.Timeout.Duration
		if timeout == 0 {
			timeout = 30 * time.Second
		}

		hookCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(hookCtx, "sh", "-c", hook.Command)
		if configDir != "" {
			cmd.Dir = configDir
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("lifecycle hook %d failed: %w", i, err)
		}

		if hook.EnvPass {
			scanner := bufio.NewScanner(&stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if idx := strings.Index(line, "="); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					val := strings.TrimSpace(line[idx+1:])
					if key != "" {
						envCopy[key] = val
					}
				}
			}
		}
	}

	return envCopy, firstErr
}

package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// BackgroundProcess tracks a background process started by a lifecycle hook.
type BackgroundProcess struct {
	Pid  int
	Cmd  *exec.Cmd
	Port int
}

var (
	backgroundProcesses []BackgroundProcess
	bgProcMu            sync.Mutex
)

// RunLifecycleHooks executes a sequence of lifecycle hooks.
// Hooks run sequentially via "sh -c <command>".
// When Background is true, the command starts non-blocking; if WaitForPort is set,
// it polls the port with exponential backoff until ready or timeout.
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

		if hook.Background {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Start(); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("lifecycle hook %d failed to start: %w", i, err)
				}
				continue
			}

			bgProcMu.Lock()
			backgroundProcesses = append(backgroundProcesses, BackgroundProcess{
				Pid:  cmd.Process.Pid,
				Cmd:  cmd,
				Port: hook.WaitForPort,
			})
			bgProcMu.Unlock()

			if hook.WaitForPort > 0 {
				startupTimeout := hook.StartupTimeout.Duration
				if startupTimeout == 0 {
					startupTimeout = timeout
				}
				if err := waitForPort(hookCtx, hook.WaitForPort, startupTimeout); err != nil {
					cmd.Process.Signal(syscall.SIGKILL)
					if firstErr == nil {
						firstErr = fmt.Errorf("lifecycle hook %d: %w", i, err)
					}
					continue
				}
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

			continue
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

// waitForPort polls a TCP port with exponential backoff until it's accepting connections.
func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 50 * time.Millisecond

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}

		sleep := backoff
		if time.Now().Add(sleep).After(deadline) {
			sleep = time.Until(deadline)
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
		backoff = backoff * 2
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
		}
	}

	return fmt.Errorf("timeout waiting for port %d after %v", port, timeout)
}

// CleanupBackgroundProcesses kills all tracked background processes.
// Sends SIGTERM, waits briefly, then SIGKILL if still alive.
func CleanupBackgroundProcesses() {
	bgProcMu.Lock()
	procs := make([]BackgroundProcess, len(backgroundProcesses))
	copy(procs, backgroundProcesses)
	bgProcMu.Unlock()

	for _, bp := range procs {
		if bp.Cmd != nil && bp.Cmd.Process != nil {
			bp.Cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	time.Sleep(100 * time.Millisecond)
	for _, bp := range procs {
		if bp.Cmd != nil && bp.Cmd.Process != nil {
			bp.Cmd.Process.Signal(syscall.SIGKILL)
		}
	}

	bgProcMu.Lock()
	backgroundProcesses = nil
	bgProcMu.Unlock()
}

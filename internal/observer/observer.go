package observer

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Observe wraps a command, captures its behavior, and returns an Observation.
func Observe(ctx context.Context, opts ObserveOptions) (*Observation, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// 1. Take pre-snapshot if Dir is set.
	var beforeSnap map[string]FileSnapshot
	if opts.Dir != "" {
		var err error
		beforeSnap, err = TakeSnapshot(opts.Dir)
		if err != nil {
			return nil, err
		}
	}

	// 2. Set up command with output capture.
	cmd := exec.CommandContext(ctx, "sh", "-c", opts.Command)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	if opts.Quiet {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	}

	start := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// 3. Signal handling: forward SIGINT/SIGTERM to child.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
	}()

	// 4. Port detection goroutine.
	var (
		portsMu sync.Mutex
		ports   []PortBinding
		wg      sync.WaitGroup
		cmdDone = make(chan struct{})
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		seen := make(map[int]bool)
		for {
			select {
			case <-ticker.C:
				if cmd.Process == nil {
					return
				}
				detected, err := DetectPorts(cmd.Process.Pid)
				if err != nil {
					continue
				}
				portsMu.Lock()
				for _, p := range detected {
					if !seen[p.Port] {
						seen[p.Port] = true
						ports = append(ports, p)
					}
				}
				portsMu.Unlock()
			case <-ctx.Done():
				return
			case <-cmdDone:
				return
			}
		}
	}()

	// 5. Wait for command to finish.
	waitErr := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)
	close(cmdDone)

	wg.Wait()

	elapsed := time.Since(start)

	// 6. Determine exit code.
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
		} else {
			exitCode = -1
		}
	}

	// 7. Take post-snapshot and diff.
	var newFiles []FileSnapshot
	if beforeSnap != nil && opts.Dir != "" {
		afterSnap, err := TakeSnapshot(opts.Dir)
		if err == nil {
			newFiles = DiffSnapshots(beforeSnap, afterSnap)
		}
	}

	// 8. Probe HTTP endpoints.
	portsMu.Lock()
	detectedPorts := ports
	portsMu.Unlock()

	var httpProbes []HTTPProbeResult
	if len(detectedPorts) > 0 {
		httpProbes = ProbeEndpoints(detectedPorts, 2*time.Second)
	}

	return &Observation{
		Command:    opts.Command,
		ExitCode:   exitCode,
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		Ports:      detectedPorts,
		NewFiles:   newFiles,
		HTTPProbes: httpProbes,
		Duration:   elapsed,
	}, nil
}

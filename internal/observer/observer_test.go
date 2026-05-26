package observer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestObserveEchoHello(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "echo hello",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", obs.ExitCode)
	}
	if !strings.Contains(obs.Stdout, "hello") {
		t.Errorf("Stdout = %q, want to contain %q", obs.Stdout, "hello")
	}
}

func TestObserveExit1(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "exit 1",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", obs.ExitCode)
	}
}

func TestObserveTimeout(t *testing.T) {
	ctx := context.Background()
	obs, err := Observe(ctx, ObserveOptions{
		Command: "sleep 30",
		Timeout: 500 * time.Millisecond,
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode == 0 {
		t.Errorf("ExitCode = %d, want non-zero (killed by timeout)", obs.ExitCode)
	}
	if obs.Duration > 2*time.Second {
		t.Errorf("Duration = %v, should be under 2s for a 500ms timeout", obs.Duration)
	}
}

func TestObserveNewFile(t *testing.T) {
	dir := t.TempDir()
	filename := "testfile_obs.txt"

	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "touch " + filename,
		Dir:     dir,
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", obs.ExitCode)
	}

	found := false
	for _, f := range obs.NewFiles {
		if filepath.Base(f.Path) == filename {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("NewFiles = %v, want to contain %q", obs.NewFiles, filename)
	}
}

func TestObserveQuietMode(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "echo quiet_output",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", obs.ExitCode)
	}
	if !strings.Contains(obs.Stdout, "quiet_output") {
		t.Errorf("Stdout = %q, want to contain %q", obs.Stdout, "quiet_output")
	}
}

func TestObserveStderr(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "echo err_msg >&2",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if !strings.Contains(obs.Stderr, "err_msg") {
		t.Errorf("Stderr = %q, want to contain %q", obs.Stderr, "err_msg")
	}
}

func TestObserveNoDir(t *testing.T) {
	obs, err := Observe(context.Background(), ObserveOptions{
		Command: "echo nodir",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", obs.ExitCode)
	}
	if obs.NewFiles != nil {
		t.Errorf("NewFiles should be nil when Dir is not set, got %v", obs.NewFiles)
	}
}

func TestObserveContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	obs, err := Observe(ctx, ObserveOptions{
		Command: "sleep 30",
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode == 0 {
		t.Errorf("ExitCode = %d, want non-zero (cancelled)", obs.ExitCode)
	}
}

func TestObserveFileContent(t *testing.T) {
	dir := t.TempDir()
	filename := "content_test.txt"

	obs, err := Observe(context.Background(), ObserveOptions{
		Command: `sh -c 'echo "test content" > ` + filename + `'`,
		Dir:     dir,
		Quiet:   true,
	})
	if err != nil {
		t.Fatalf("Observe returned error: %v", err)
	}
	if obs.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", obs.ExitCode)
	}

	// Verify the file was actually created.
	if _, err := os.Stat(filepath.Join(dir, filename)); os.IsNotExist(err) {
		t.Errorf("file %q not created in dir %s", filename, dir)
	}
}

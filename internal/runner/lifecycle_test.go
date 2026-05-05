package runner

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestLifecycle_BeforeAllRuns(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo before-all-ran"},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if newEnv == nil {
		t.Fatal("expected env map to be returned")
	}
}

func TestLifecycle_AfterAllRuns(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo after-all-ran"},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if newEnv == nil {
		t.Fatal("expected env map to be returned")
	}
}

func TestLifecycle_AfterAllRunsOnFailure(t *testing.T) {
	// Create a temp file that will be touched by after_all with always_run=true
	tmpDir := t.TempDir()
	flagFile := tmpDir + "/after_all_ran"

	hooks := []schema.LifecycleHook{
		{Command: "exit 1"}, // Simulate test failure
		{Command: "touch " + flagFile, AlwaysRun: true},
	}

	env := make(map[string]string)
	_, _ = RunLifecycleHooks(context.Background(), hooks, env, "")

	// Check if the always_run hook executed despite previous failure
	if _, err := os.Stat(flagFile); os.IsNotExist(err) {
		t.Error("after_all hook with always_run=true should execute even when previous hooks fail")
	}
}

func TestLifecycle_BeforeEachRuns(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo before-each-ran"},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if newEnv == nil {
		t.Fatal("expected env map to be returned")
	}
}

func TestLifecycle_AfterEachRuns(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo after-each-ran"},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if newEnv == nil {
		t.Fatal("expected env map to be returned")
	}
}

func TestLifecycle_EnvPass(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo TEST_VAR=test_value", EnvPass: true},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["TEST_VAR"]; !ok {
		t.Error("expected TEST_VAR to be set in env")
	} else if val != "test_value" {
		t.Errorf("expected TEST_VAR=test_value, got %s", val)
	}
}

func TestLifecycle_BeforeAllFailureSkipsTests(t *testing.T) {
	// This tests the integration behavior where before_all failure
	// causes tests to be skipped. The actual implementation will be
	// in runner.go, but we verify the error propagation here.

	hooks := []schema.LifecycleHook{
		{Command: "exit 1"},
	}

	env := make(map[string]string)
	_, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err == nil {
		t.Error("expected error from failing before_all hook, got nil")
	}
}

func TestLifecycle_EmptyLifecycle(t *testing.T) {
	// No hooks means no errors and no env changes
	var hooks []schema.LifecycleHook

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error with empty lifecycle, got %v", err)
	}

	if newEnv == nil {
		t.Fatal("expected env map to be returned")
	}

	if len(newEnv) != 0 {
		t.Errorf("expected empty env map, got %d entries", len(newEnv))
	}
}

func TestLifecycle_MultipleEnvVars(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo FIRST=1 && echo SECOND=2", EnvPass: true},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["FIRST"]; !ok || val != "1" {
		t.Errorf("expected FIRST=1, got %s (ok: %v)", val, ok)
	}
	if val, ok := newEnv["SECOND"]; !ok || val != "2" {
		t.Errorf("expected SECOND=2, got %s (ok: %v)", val, ok)
	}
}

func TestLifecycle_HookTimeout(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "sleep 10", Timeout: schema.Duration{Duration: 100 * time.Millisecond}},
	}

	env := make(map[string]string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RunLifecycleHooks(ctx, hooks, env, "")

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestLifecycle_EnvPassIgnoresNonKeyValueLines(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo random output\necho VALID=value\necho more junk", EnvPass: true},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["VALID"]; !ok || val != "value" {
		t.Errorf("expected VALID=value, got %s (ok: %v)", val, ok)
	}

	// Non-KEY=VALUE lines should not create env vars
	if _, ok := newEnv["random"]; ok {
		t.Error("expected 'random' to not be set as env var")
	}
	if _, ok := newEnv["more"]; ok {
		t.Error("expected 'more' to not be set as env var")
	}
}

func TestLifecycle_HookRespectsConfigDir(t *testing.T) {
	// Create a temp directory with a script
	tmpDir := t.TempDir()
	scriptPath := tmpDir + "/test.sh"

	// Create a simple script that echoes a known value
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho config_dir_test=ok"), 0755); err != nil {
		t.Fatal(err)
	}

	hooks := []schema.LifecycleHook{
		{Command: "./test.sh", EnvPass: true},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["config_dir_test"]; !ok || val != "ok" {
		t.Errorf("expected config_dir_test=ok, got %s (ok: %v)", val, ok)
	}
}

func TestLifecycle_EnvOverwritesExisting(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo EXISTING=new_value", EnvPass: true},
	}

	env := map[string]string{"EXISTING": "old_value"}
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["EXISTING"]; !ok || val != "new_value" {
		t.Errorf("expected EXISTING=new_value, got %s (ok: %v)", val, ok)
	}
}

func TestLifecycle_AlwaysRunStopsAtFirstErrorWithoutFlag(t *testing.T) {
	tmpDir := t.TempDir()
	firstFile := tmpDir + "/first"
	secondFile := tmpDir + "/second"

	hooks := []schema.LifecycleHook{
		{Command: "touch " + firstFile},
		{Command: "exit 1"},
		{Command: "touch " + secondFile, AlwaysRun: true}, // Should run since always_run=true
	}

	env := make(map[string]string)
	RunLifecycleHooks(context.Background(), hooks, env, "")

	// First hook should have run
	if _, err := os.Stat(firstFile); os.IsNotExist(err) {
		t.Error("expected first hook to run")
	}

	// Third hook with always_run=true should have run
	if _, err := os.Stat(secondFile); os.IsNotExist(err) {
		t.Error("expected third hook with always_run=true to run even after error")
	}
}

func TestLifecycle_EnvPassWithSpaces(t *testing.T) {
	hooks := []schema.LifecycleHook{
		{Command: "echo VALUE=hello world", EnvPass: true},
	}

	env := make(map[string]string)
	newEnv, err := RunLifecycleHooks(context.Background(), hooks, env, "")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if val, ok := newEnv["VALUE"]; !ok || val != "hello world" {
		t.Errorf("expected VALUE='hello world', got %s (ok: %v)", val, ok)
	}
}


func TestRunLifecycleHooks_BackgroundStarts(t *testing.T) {
	backgroundProcesses = nil
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "sleep 10", Background: true, Timeout: schema.Duration{Duration: 5 * time.Second}},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(backgroundProcesses) == 0 {
		t.Fatal("expected background process to be tracked")
	}
	CleanupBackgroundProcesses()
}

func TestRunLifecycleHooks_WaitForPortReady(t *testing.T) {
	backgroundProcesses = nil
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()
	defer listener.Close()

	err = waitForPort(context.Background(), port, 2*time.Second)
	if err != nil {
		t.Fatalf("expected port to be ready: %v", err)
	}
}

func TestRunLifecycleHooks_WaitForPortTimeout(t *testing.T) {
	err := waitForPort(context.Background(), 59999, 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestRunLifecycleHooks_BackgroundCleanup(t *testing.T) {
	backgroundProcesses = nil
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "sleep 30", Background: true, Timeout: schema.Duration{Duration: 5 * time.Second}},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(backgroundProcesses) == 0 {
		t.Fatal("expected background process")
	}

	CleanupBackgroundProcesses()

	if len(backgroundProcesses) != 0 {
		t.Fatal("expected background processes to be cleared")
	}
}

func TestRunLifecycleHooks_BackgroundWithoutWaitForPort(t *testing.T) {
	backgroundProcesses = nil
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "sleep 5", Background: true, Timeout: schema.Duration{Duration: 10 * time.Second}},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	CleanupBackgroundProcesses()
}

func TestRunLifecycleHooks_PortPollingBackoff(t *testing.T) {
	start := time.Now()
	err := waitForPort(context.Background(), 59998, 300*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout")
	}
	if elapsed < 280*time.Millisecond {
		t.Fatalf("polling exited too quickly: %v", elapsed)
	}
}

func TestRunLifecycleHooks_EnvPassFromBackground(t *testing.T) {
	backgroundProcesses = nil
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "echo PORT=8080", EnvPass: true},
	}
	env, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["PORT"] != "8080" {
		t.Fatalf("expected PORT=8080 in env, got: %v", env)
	}
}

func TestRunLifecycleHooks_BackgroundWithAlwaysRun(t *testing.T) {
	backgroundProcesses = nil
	ctx := context.Background()
	hooks := []schema.LifecycleHook{
		{Command: "sleep 5", Background: true, AlwaysRun: true, Timeout: schema.Duration{Duration: 10 * time.Second}},
	}
	_, err := RunLifecycleHooks(ctx, hooks, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	CleanupBackgroundProcesses()
}

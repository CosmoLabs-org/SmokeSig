package plugin

import (
	"context"
	"testing"
)

func TestSandboxCheck(t *testing.T) {
	s := NewSandbox("test-plugin", []string{"network", "env"}, 16)

	// Granted capabilities should pass
	if err := s.Check("network"); err != nil {
		t.Errorf("expected network check to pass, got: %v", err)
	}
	if err := s.Check("env"); err != nil {
		t.Errorf("expected env check to pass, got: %v", err)
	}

	// Ungranted capabilities should fail
	if err := s.Check("time"); err == nil {
		t.Error("expected time check to fail")
	} else {
		pe, ok := err.(*PluginError)
		if !ok {
			t.Errorf("expected PluginError, got: %T", err)
		} else if pe.Kind != ErrCapabilityDeny {
			t.Errorf("expected ErrCapabilityDeny, got: %v", pe.Kind)
		}
	}

	if err := s.Check("fs_read"); err == nil {
		t.Error("expected fs_read check to fail")
	}
	if err := s.Check("exec"); err == nil {
		t.Error("expected exec check to fail")
	}
}

func TestSandboxEmpty(t *testing.T) {
	s := NewSandbox("empty", nil, 0)

	// Empty capabilities should deny everything
	for _, cap := range []string{"network", "env", "time", "fs_read", "exec"} {
		if err := s.Check(cap); err == nil {
			t.Errorf("expected %s check to fail with empty capabilities", cap)
		}
	}
}

func TestSandboxMemoryLimitPages(t *testing.T) {
	// 16MB = 256 pages (16 * 16)
	s := NewSandbox("test", nil, 16)
	if pages := s.MemoryLimitPages(); pages != 256 {
		t.Errorf("expected 256 pages for 16MB, got: %d", pages)
	}

	// 32MB = 512 pages
	s2 := NewSandbox("test", nil, 32)
	if pages := s2.MemoryLimitPages(); pages != 512 {
		t.Errorf("expected 512 pages for 32MB, got: %d", pages)
	}
}

func TestSandboxDefaultMemory(t *testing.T) {
	s := NewSandbox("test", nil, 0)
	// Default should be 16MB = 256 pages
	if pages := s.MemoryLimitPages(); pages != 256 {
		t.Errorf("expected 256 pages for default memory, got: %d", pages)
	}
}

func TestSandboxContext(t *testing.T) {
	s := NewSandbox("ctx-test", []string{"network"}, 16)
	ctx := context.Background()

	// No sandbox in context
	if got := SandboxFromContext(ctx); got != nil {
		t.Error("expected nil sandbox from empty context")
	}

	// With sandbox in context
	ctx = ContextWithSandbox(ctx, s)
	got := SandboxFromContext(ctx)
	if got == nil {
		t.Fatal("expected sandbox from context")
	}
	if err := got.Check("network"); err != nil {
		t.Errorf("expected network check to pass: %v", err)
	}
}

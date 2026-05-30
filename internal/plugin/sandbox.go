package plugin

import (
	"context"
	"fmt"
	"sort"
)

// ValidCapabilities is the set of recognized capability names.
var ValidCapabilities = map[string]bool{
	"network": true,
	"env":     true,
	"time":    true,
	"fs_read": true,
	"exec":    true,
}

// Sandbox enforces capability-based access control for a plugin instance.
type Sandbox struct {
	pluginName    string
	capabilities  map[string]bool
	memoryLimitMB int
}

// NewSandbox creates a sandbox from the plugin's granted capabilities.
func NewSandbox(name string, capabilities []string, memoryLimitMB int) *Sandbox {
	caps := make(map[string]bool, len(capabilities))
	for _, c := range capabilities {
		caps[c] = true
	}
	if memoryLimitMB <= 0 {
		memoryLimitMB = DefaultMemoryLimitMB
	}
	return &Sandbox{
		pluginName:    name,
		capabilities:  caps,
		memoryLimitMB: memoryLimitMB,
	}
}

// Check returns nil if the capability is granted, or a PluginError if denied.
func (s *Sandbox) Check(capability string) error {
	if s.capabilities[capability] {
		return nil
	}
	return &PluginError{
		Kind:    ErrCapabilityDeny,
		Message: fmt.Sprintf("plugin %q called %s but capability not granted (granted: %v)", s.pluginName, capability, s.grantedList()),
	}
}

// MemoryLimitPages returns the wazero memory limit in pages (64KB each).
func (s *Sandbox) MemoryLimitPages() uint32 {
	return uint32(s.memoryLimitMB) * 16 // 1MB = 16 pages of 64KB
}

func (s *Sandbox) grantedList() []string {
	var list []string
	for c := range s.capabilities {
		list = append(list, c)
	}
	sort.Strings(list)
	return list
}

// sandboxKey is the context key for sandbox dispatch.
type sandboxKey struct{}

// SandboxFromContext retrieves the active sandbox from context.
func SandboxFromContext(ctx context.Context) *Sandbox {
	s, _ := ctx.Value(sandboxKey{}).(*Sandbox)
	return s
}

// ContextWithSandbox stores a sandbox in context for host function dispatch.
func ContextWithSandbox(ctx context.Context, s *Sandbox) context.Context {
	return context.WithValue(ctx, sandboxKey{}, s)
}

package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// PluginManager loads, caches, and evaluates Wasm plugins.
type PluginManager struct {
	runtime   wazero.Runtime
	compiled  map[string]wazero.CompiledModule // keyed by SHA-256 of .wasm bytes
	configs   map[string]*PluginConfig         // keyed by plugin name
	sandboxes map[string]*Sandbox              // keyed by plugin name
	mu        sync.RWMutex
	debug     bool
	configDir string // base directory for resolving relative plugin paths
}

// ManagerOptions configures PluginManager creation.
type ManagerOptions struct {
	ConfigDir     string
	CacheDir      string // filesystem compilation cache (empty = no cache)
	MemoryLimitMB int    // per-module memory limit (0 = default 16MB)
	Debug         bool   // log plugin I/O to stderr
}

// NewPluginManager creates a PluginManager with a wazero runtime.
func NewPluginManager(ctx context.Context, opts ManagerOptions) (*PluginManager, error) {
	runtimeCfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true)

	if opts.CacheDir != "" {
		if err := os.MkdirAll(opts.CacheDir, 0755); err == nil {
			cache, err := wazero.NewCompilationCacheWithDir(opts.CacheDir)
			if err == nil {
				runtimeCfg = runtimeCfg.WithCompilationCache(cache)
			}
		}
	}

	rt := wazero.NewRuntimeWithConfig(ctx, runtimeCfg)

	// Instantiate WASI for ABI v1 modules
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiating WASI: %w", err)
	}

	// Register host functions once (sandbox resolved per-invocation via context)
	if _, err := RegisterHostFunctions(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("registering host functions: %w", err)
	}

	return &PluginManager{
		runtime:   rt,
		compiled:  make(map[string]wazero.CompiledModule),
		configs:   make(map[string]*PluginConfig),
		sandboxes: make(map[string]*Sandbox),
		debug:     opts.Debug || os.Getenv("SMOKESIG_PLUGIN_DEBUG") == "1",
		configDir: opts.ConfigDir,
	}, nil
}

// LoadPlugin compiles and registers a plugin by name.
func (m *PluginManager) LoadPlugin(ctx context.Context, name string, entry schema.PluginEntry, memoryLimitMB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve path relative to config directory
	wasmPath := entry.Path
	if !filepath.IsAbs(wasmPath) {
		wasmPath = filepath.Join(m.configDir, wasmPath)
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q: %v", name, err), Cause: err}
	}

	// SHA-256 for compilation cache key
	hash := sha256.Sum256(wasmBytes)
	hashStr := hex.EncodeToString(hash[:])

	// Check compilation cache
	compiled, ok := m.compiled[hashStr]
	if !ok {
		compiled, err = m.runtime.CompileModule(ctx, wasmBytes)
		if err != nil {
			return &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("plugin %q: compilation failed: %v", name, err), Cause: err}
		}
		m.compiled[hashStr] = compiled
	}

	// Detect ABI version
	abiVersion, err := DetectABI(compiled)
	if err != nil {
		return err
	}

	timeout := entry.Timeout.Duration
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	config := &PluginConfig{
		Name:         name,
		Path:         wasmPath,
		Timeout:      timeout,
		Capabilities: entry.Capabilities,
		ABIVersion:   abiVersion,
	}
	m.configs[name] = config

	sandbox := NewSandbox(name, entry.Capabilities, memoryLimitMB)
	m.sandboxes[name] = sandbox

	if m.debug {
		fmt.Fprintf(os.Stderr, "[plugin-debug] loaded %q (ABI v%d, hash %s, caps %v)\n", name, abiVersion, hashStr[:12], entry.Capabilities)
	}

	return nil
}

// Evaluate runs a plugin assertion and returns the result.
func (m *PluginManager) Evaluate(ctx context.Context, name string, input PluginInput) (*PluginOutput, error) {
	m.mu.RLock()
	config, ok := m.configs[name]
	sandbox := m.sandboxes[name]
	m.mu.RUnlock()

	if !ok {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q not loaded", name)}
	}

	// Apply timeout
	timeout := config.Timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Set ABI version from detected config
	input.ABIVersion = config.ABIVersion

	// Inject sandbox into context for host function dispatch
	ctx = ContextWithSandbox(ctx, sandbox)

	if m.debug {
		inputJSON, _ := json.Marshal(input)
		fmt.Fprintf(os.Stderr, "[plugin-debug] evaluate %q input: %s\n", name, string(inputJSON))
	}

	var output *PluginOutput
	var evalErr error

	// Find compiled module by looking up config path hash
	wasmBytes, readErr := os.ReadFile(config.Path)
	if readErr != nil {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q .wasm file disappeared: %v", name, readErr)}
	}
	hash := sha256.Sum256(wasmBytes)
	hashStr := hex.EncodeToString(hash[:])

	m.mu.RLock()
	compiled, ok := m.compiled[hashStr]
	m.mu.RUnlock()
	if !ok {
		return nil, &PluginError{Kind: ErrNotFound, Message: fmt.Sprintf("plugin %q not compiled", name)}
	}

	switch config.ABIVersion {
	case ABIVersionMemory:
		// Instantiate a fresh module for this invocation
		modConfig := wazero.NewModuleConfig().WithName("")
		mod, instErr := m.runtime.InstantiateModule(ctx, compiled, modConfig)
		if instErr != nil {
			return nil, classifyWazeroError(instErr)
		}
		defer mod.Close(ctx)
		output, evalErr = InvokeMemoryABI(ctx, mod, &input)

	case ABIVersionWASI:
		output, evalErr = InvokeWASIABI(ctx, m.runtime, compiled, &input)

	default:
		return nil, &PluginError{Kind: ErrABIMismatch, Message: fmt.Sprintf("unsupported ABI version %d", config.ABIVersion)}
	}

	if m.debug && output != nil {
		outputJSON, _ := json.Marshal(output)
		fmt.Fprintf(os.Stderr, "[plugin-debug] evaluate %q output: %s\n", name, string(outputJSON))
	}

	return output, evalErr
}

// Close releases all compiled modules and the wazero runtime.
func (m *PluginManager) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

// PluginNames returns the sorted list of loaded plugin names.
func (m *PluginManager) PluginNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

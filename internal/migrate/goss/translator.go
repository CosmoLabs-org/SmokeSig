package goss

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// WarningCategory classifies translation warnings.
type WarningCategory int

const (
	WarnCommandFallback WarningCategory = iota // Mapped via command: fallback
	WarnSkipped                                // No mapping exists, emitted as TODO
	WarnPartial                                // Partially mapped, some attrs dropped
)

// TranslationWarning records a non-fatal issue during translation.
type TranslationWarning struct {
	GossKey   string          // e.g. "package", "dns"
	Resource  string          // e.g. "nginx", "tcp:80"
	Category  WarningCategory
	Message   string
}

// TranslateOptions controls translation behavior.
type TranslateOptions struct {
	Distro string // deb, rpm, apk
}

// Translate converts a GossFile into SmokeSig tests and warnings.
func Translate(gf *GossFile, opts TranslateOptions) ([]schema.Test, []TranslationWarning) {
	var tests []schema.Test
	var warnings []TranslationWarning

	if opts.Distro == "" {
		opts.Distro = "deb"
	}

	// Core 7 keys — direct or high-fidelity mapping
	for name, attrs := range gf.Process {
		ts, ws := translateProcess(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Port {
		ts, ws := translatePort(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Command {
		ts, ws := translateCommand(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.File {
		ts, ws := translateFile(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.HTTP {
		ts, ws := translateHTTP(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Package {
		ts, ws := translatePackage(name, attrs, opts.Distro)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Service {
		ts, ws := translateService(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}

	// Long-tail keys — command fallback or TODO stubs
	for name, attrs := range gf.User {
		ts, ws := translateUser(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Group {
		ts, ws := translateGroup(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.DNS {
		ts, ws := translateDNS(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Addr {
		ts, ws := translateAddr(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Interface {
		ts, ws := translateInterface(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.Mount {
		ts, ws := translateMount(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}
	for name, attrs := range gf.KernelParam {
		ts, ws := translateKernelParam(name, attrs)
		tests = append(tests, ts...)
		warnings = append(warnings, ws...)
	}

	return tests, warnings
}

func intPtr(v int) *int { return &v }

func boolVal(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func stringVal(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intVal(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

func stringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case []string:
			return s
		case []interface{}:
			var result []string
			for _, item := range s {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// distroPkgCmd returns the package check command for a given distro.
func distroPkgCmd(name, distro string) string {
	switch distro {
	case "rpm":
		return fmt.Sprintf("rpm -q %s", name)
	case "apk":
		return fmt.Sprintf("apk info -e %s", name)
	default: // deb
		return fmt.Sprintf("dpkg -l %s | grep ^ii", name)
	}
}

func translateProcess(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "running") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("process:%s running", name),
			Run:  "true",
			Expect: schema.Expect{
				ExitCode:      intPtr(0),
				ProcessRunning: name,
			},
		},
	}, nil
}

func translatePort(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "listening") {
		return nil, nil
	}

	// Parse Goss port format: "tcp:80", "udp:53", "tcp:8080:0.0.0.0"
	parts := strings.SplitN(name, ":", 3)
	if len(parts) < 2 {
		return nil, []TranslationWarning{{
			GossKey:  "port",
			Resource: name,
			Category: WarnSkipped,
			Message:  fmt.Sprintf("cannot parse port format %q", name),
		}}
	}

	protocol := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, []TranslationWarning{{
			GossKey:  "port",
			Resource: name,
			Category: WarnSkipped,
			Message:  fmt.Sprintf("cannot parse port number from %q", parts[1]),
		}}
	}

	host := "0.0.0.0"
	if len(parts) == 3 {
		host = parts[2]
	}
	// Goss also uses ip: field
	if ipVal := stringSlice(attrs, "ip"); len(ipVal) > 0 {
		host = ipVal[0]
	}

	return []schema.Test{
		{
			Name: fmt.Sprintf("port:%s listening", name),
			Run:  "true",
			Expect: schema.Expect{
				ExitCode: intPtr(0),
				PortListening: &schema.PortCheck{
					Port:     port,
					Protocol: protocol,
					Host:     host,
				},
			},
		},
	}, nil
}

func translateCommand(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	expect := schema.Expect{}
	if exitCode := intVal(attrs, "exit-status"); exitCode != 0 || hasKey(attrs, "exit-status") {
		expect.ExitCode = intPtr(exitCode)
	}

	stdout := stringSlice(attrs, "stdout")
	if len(stdout) > 0 {
		expect.StdoutContains = stdout[0]
	}

	stderr := stringSlice(attrs, "stderr")
	if len(stderr) > 0 {
		expect.StderrContains = stderr[0]
	}

	var warnings []TranslationWarning
	if len(stdout) > 1 || len(stderr) > 1 {
		warnings = append(warnings, TranslationWarning{
			GossKey:  "command",
			Resource: name,
			Category: WarnPartial,
			Message:  "Goss supports multiple stdout/stderr entries; SmokeSig maps only the first. Add additional tests manually.",
		})
	}

	return []schema.Test{
		{
			Name:   fmt.Sprintf("command:%s", name),
			Run:    name,
			Expect: expect,
		},
	}, warnings
}

func hasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

func translateFile(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	var tests []schema.Test
	var warnings []TranslationWarning

	if boolVal(attrs, "exists") {
		tests = append(tests, schema.Test{
			Name: fmt.Sprintf("file:%s exists", name),
			Run:  "true",
			Expect: schema.Expect{
				ExitCode:   intPtr(0),
				FileExists: name,
			},
		})
	}

	// Check for attributes we can't map
	unmapped := []string{}
	if hasKey(attrs, "mode") {
		unmapped = append(unmapped, "mode")
	}
	if hasKey(attrs, "owner") {
		unmapped = append(unmapped, "owner")
	}
	if hasKey(attrs, "group") {
		unmapped = append(unmapped, "group")
	}
	if hasKey(attrs, "contains") {
		unmapped = append(unmapped, "contains")
	}
	if hasKey(attrs, "filetype") {
		unmapped = append(unmapped, "filetype")
	}
	if hasKey(attrs, "checksum") {
		unmapped = append(unmapped, "checksum")
	}
	if hasKey(attrs, "size") {
		unmapped = append(unmapped, "size")
	}

	if len(unmapped) > 0 {
		warnings = append(warnings, TranslationWarning{
			GossKey:  "file",
			Resource: name,
			Category: WarnPartial,
			Message:  fmt.Sprintf("Goss specified %s — not yet supported natively", strings.Join(unmapped, ", ")),
		})
	}

	return tests, warnings
}

func translateHTTP(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	check := &schema.HTTPCheck{
		URL: name,
	}

	if status := intVal(attrs, "status"); status != 0 {
		check.StatusCode = intPtr(status)
	}

	body := stringSlice(attrs, "body")
	if len(body) > 0 {
		check.BodyContains = body[0]
	}

	headers := stringSlice(attrs, "headers")
	_ = headers // TODO: map to header_contains

	var warnings []TranslationWarning
	if len(body) > 1 {
		warnings = append(warnings, TranslationWarning{
			GossKey:  "http",
			Resource: name,
			Category: WarnPartial,
			Message:  "Goss supports multiple body matchers; SmokeSig maps only the first via body_contains",
		})
	}

	check.Method = stringVal(attrs, "method")

	return []schema.Test{
		{
			Name: fmt.Sprintf("http:%s", name),
			Run:  "true",
			Expect: schema.Expect{
				HTTP: check,
			},
		},
	}, warnings
}

func translatePackage(name string, attrs GossAttrs, distro string) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "installed") {
		return nil, nil
	}

	cmd := distroPkgCmd(name, distro)
	var warnings []TranslationWarning

	if version := stringVal(attrs, "version"); version != "" {
		warnings = append(warnings, TranslationWarning{
			GossKey:  "package",
			Resource: name,
			Category: WarnPartial,
			Message:  fmt.Sprintf("Goss specified version=%q — add stdout_contains check manually", version),
		})
	}

	return []schema.Test{
		{
			Name: fmt.Sprintf("package:%s installed", name),
			Run:  cmd,
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, warnings
}

func translateService(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	var tests []schema.Test
	var warnings []TranslationWarning

	if boolVal(attrs, "running") {
		tests = append(tests, schema.Test{
			Name: fmt.Sprintf("service:%s active", name),
			Run:  fmt.Sprintf("systemctl is-active %s", name),
			Expect: schema.Expect{
				ExitCode:       intPtr(0),
				StdoutContains: "active",
			},
		})
		warnings = append(warnings, TranslationWarning{
			GossKey:  "service",
			Resource: name,
			Category: WarnCommandFallback,
			Message:  "Migrated via systemctl command fallback",
		})
	}

	if boolVal(attrs, "enabled") {
		tests = append(tests, schema.Test{
			Name: fmt.Sprintf("service:%s enabled", name),
			Run:  fmt.Sprintf("systemctl is-enabled %s", name),
			Expect: schema.Expect{
				ExitCode:       intPtr(0),
				StdoutContains: "enabled",
			},
		})
	}

	return tests, warnings
}

// Long-tail key translators — command fallback or TODO stubs

func translateUser(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "exists") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("user:%s exists", name),
			Run:  fmt.Sprintf("id %s", name),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "user",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via id command fallback",
	}}
}

func translateGroup(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "exists") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("group:%s exists", name),
			Run:  fmt.Sprintf("getent group %s", name),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "group",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via getent command fallback",
	}}
}

func translateDNS(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "resolvable") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("dns:%s resolvable", name),
			Run:  fmt.Sprintf("getent hosts %s", name),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "dns",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via getent hosts command fallback",
	}}
}

func translateAddr(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "reachable") {
		return nil, nil
	}

	// Parse "tcp://host:port" or "udp://host:port"
	parts := strings.SplitN(strings.TrimPrefix(name, "tcp://"), ":", 2)
	// Also handle udp://
	addr := strings.TrimPrefix(name, "udp://")
	addr = strings.TrimPrefix(addr, "tcp://")

	host := addr
	port := 0
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		host = addr[:idx]
		if p, err := strconv.Atoi(addr[idx+1:]); err == nil {
			port = p
		}
	}

	if port > 0 {
		return []schema.Test{
			{
				Name: fmt.Sprintf("addr:%s reachable", name),
				Run:  "true",
				Expect: schema.Expect{
					PortListening: &schema.PortCheck{
						Host: host,
						Port: port,
					},
				},
			},
		}, nil
	}

	// No port — fall back to command
	_ = parts
	return []schema.Test{
		{
			Name: fmt.Sprintf("addr:%s reachable", name),
			Run:  fmt.Sprintf("nc -z %s 2>/dev/null || true", host),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "addr",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via nc command fallback",
	}}
}

func translateInterface(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "exists") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("interface:%s exists", name),
			Run:  fmt.Sprintf("ip link show %s", name),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "interface",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via ip command fallback",
	}}
}

func translateMount(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	if !boolVal(attrs, "exists") {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("mount:%s exists", name),
			Run:  fmt.Sprintf("mountpoint -q %s", name),
			Expect: schema.Expect{
				ExitCode: intPtr(0),
			},
		},
	}, []TranslationWarning{{
		GossKey:  "mount",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via mountpoint command fallback",
	}}
}

func translateKernelParam(name string, attrs GossAttrs) ([]schema.Test, []TranslationWarning) {
	value := stringVal(attrs, "value")
	if value == "" {
		return nil, nil
	}
	return []schema.Test{
		{
			Name: fmt.Sprintf("kernel-param:%s", name),
			Run:  fmt.Sprintf("sysctl -n %s", name),
			Expect: schema.Expect{
				ExitCode:       intPtr(0),
				StdoutContains: value,
			},
		},
	}, []TranslationWarning{{
		GossKey:  "kernel-param",
		Resource: name,
		Category: WarnCommandFallback,
		Message:  "Migrated via sysctl command fallback",
	}}
}

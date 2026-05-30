package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const hostModuleName = "smokesig"

// HostResponse is the JSON format for host function results written back to guest memory.
type HostResponse struct {
	Status int    `json:"status,omitempty"`
	Body   string `json:"body,omitempty"`
	Error  string `json:"error,omitempty"`
	Value  string `json:"value,omitempty"`
	TimeMs int64  `json:"time_ms,omitempty"`
}

// RegisterHostFunctions registers the "smokesig" host module with wazero.
// Called once in NewPluginManager. Each host function extracts the active
// sandbox from context via SandboxFromContext(ctx) for capability checks.
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime) (wazero.CompiledModule, error) {
	builder := runtime.NewHostModuleBuilder(hostModuleName)

	builder.NewFunctionBuilder().
		WithFunc(makeHTTPGet()).
		WithParameterNames("url_ptr", "url_len", "headers_ptr", "headers_len").
		Export("host_http_get")

	builder.NewFunctionBuilder().
		WithFunc(makeHTTPPost()).
		WithParameterNames("url_ptr", "url_len", "body_ptr", "body_len", "headers_ptr", "headers_len").
		Export("host_http_post")

	builder.NewFunctionBuilder().
		WithFunc(makeEnvGet()).
		WithParameterNames("name_ptr", "name_len").
		Export("host_env_get")

	builder.NewFunctionBuilder().
		WithFunc(makeTimeNow()).
		Export("host_time_now")

	compiled, err := builder.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compiling host module: %w", err)
	}
	return compiled, nil
}

// makeHTTPGet returns a host function that performs an HTTP GET.
// Requires "network" capability. Reads URL from guest memory, writes response JSON back.
func makeHTTPGet() func(ctx context.Context, mod api.Module, urlPtr, urlLen, headersPtr, headersLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, urlPtr, urlLen, headersPtr, headersLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("network"); err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: err.Error()})
		}

		urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
		if !ok {
			return writeHostResponse(ctx, mod, HostResponse{Error: "failed to read URL from memory"})
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(urlBytes), nil)
		if err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: fmt.Sprintf("invalid URL: %v", err)})
		}

		// Parse headers if provided
		if headersLen > 0 {
			hdrBytes, ok := mod.Memory().Read(headersPtr, headersLen)
			if ok {
				var headers map[string]string
				if json.Unmarshal(hdrBytes, &headers) == nil {
					for k, v := range headers {
						req.Header.Set(k, v)
					}
				}
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: fmt.Sprintf("HTTP GET failed: %v", err)})
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return writeHostResponse(ctx, mod, HostResponse{Status: resp.StatusCode, Body: string(body)})
	}
}

// makeHTTPPost returns a host function that performs an HTTP POST.
// Requires "network" capability.
func makeHTTPPost() func(ctx context.Context, mod api.Module, urlPtr, urlLen, bodyPtr, bodyLen, headersPtr, headersLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, urlPtr, urlLen, bodyPtr, bodyLen, headersPtr, headersLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("network"); err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: err.Error()})
		}

		urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
		if !ok {
			return writeHostResponse(ctx, mod, HostResponse{Error: "failed to read URL from memory"})
		}

		var bodyReader io.Reader
		if bodyLen > 0 {
			bodyBytes, ok := mod.Memory().Read(bodyPtr, bodyLen)
			if !ok {
				return writeHostResponse(ctx, mod, HostResponse{Error: "failed to read body from memory"})
			}
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, string(urlBytes), bodyReader)
		if err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: fmt.Sprintf("invalid URL: %v", err)})
		}

		if headersLen > 0 {
			hdrBytes, ok := mod.Memory().Read(headersPtr, headersLen)
			if ok {
				var headers map[string]string
				if json.Unmarshal(hdrBytes, &headers) == nil {
					for k, v := range headers {
						req.Header.Set(k, v)
					}
				}
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: fmt.Sprintf("HTTP POST failed: %v", err)})
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return writeHostResponse(ctx, mod, HostResponse{Status: resp.StatusCode, Body: string(body)})
	}
}

// makeEnvGet returns a host function that reads an environment variable.
// Requires "env" capability.
func makeEnvGet() func(ctx context.Context, mod api.Module, namePtr, nameLen uint32) uint64 {
	return func(ctx context.Context, mod api.Module, namePtr, nameLen uint32) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: "no sandbox in context"})
		}
		if err := sandbox.Check("env"); err != nil {
			return writeHostResponse(ctx, mod, HostResponse{Error: err.Error()})
		}

		nameBytes, ok := mod.Memory().Read(namePtr, nameLen)
		if !ok {
			return writeHostResponse(ctx, mod, HostResponse{Error: "failed to read env var name from memory"})
		}

		value := os.Getenv(string(nameBytes))
		return writeHostResponse(ctx, mod, HostResponse{Value: value})
	}
}

// makeTimeNow returns a host function that returns current time in unix millis.
// Requires "time" capability.
func makeTimeNow() func(ctx context.Context, mod api.Module) uint64 {
	return func(ctx context.Context, mod api.Module) uint64 {
		sandbox := SandboxFromContext(ctx)
		if sandbox == nil {
			return 0
		}
		if err := sandbox.Check("time"); err != nil {
			return 0
		}
		return uint64(time.Now().UnixMilli())
	}
}

// writeHostResponse marshals a HostResponse to JSON and writes it to guest memory
// via smokesig_alloc, returning (ptr << 32) | len.
func writeHostResponse(ctx context.Context, mod api.Module, resp HostResponse) uint64 {
	data, err := json.Marshal(resp)
	if err != nil {
		return 0
	}

	alloc := mod.ExportedFunction("smokesig_alloc")
	if alloc == nil {
		return 0
	}

	results, err := alloc.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0
	}
	ptr := uint32(results[0])

	if !mod.Memory().Write(ptr, data) {
		return 0
	}

	return (uint64(ptr) << 32) | uint64(len(data))
}

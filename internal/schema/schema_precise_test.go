package schema

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFetchHTTP_NewRequestError covers remote.go:71-73 (http.NewRequestWithContext error).
// A URL containing a null byte is syntactically parseable but fails NewRequestWithContext.
// The cacheDir MkdirAll and readCache (no-exist) succeed first, so NewRequest is reached.
func TestFetchHTTP_NewRequestError(t *testing.T) {
	r := NewRemoteResolver(t.TempDir())
	// Null byte in URL causes NewRequestWithContext to return an error.
	_, err := r.fetchHTTP(context.Background(), "http://host/\x00path")
	if err == nil {
		t.Fatal("expected error from NewRequestWithContext with null byte URL, got nil")
	}
	if !strings.Contains(err.Error(), "creating request") {
		t.Errorf("expected 'creating request' in error, got: %v", err)
	}
}

// TestFetchHTTP_ReadBodyErrorNoCache covers remote.go:111 — io.ReadAll fails with no cached body.
// We use a raw TCP server that sends a valid HTTP 200 header but then closes the connection
// immediately, causing io.ReadAll to get an unexpected EOF with no cached body → error returned.
func TestFetchHTTP_ReadBodyErrorNoCache(t *testing.T) {
	// Start a raw TCP listener that sends 200 OK header then immediately closes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Send a valid HTTP response header claiming content but no body.
		// Content-Length > 0 makes io.ReadAll expect bytes, then gets EOF → error.
		fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/yaml\r\nContent-Length: 100\r\n\r\n")
		// Close without sending body — io.ReadAll gets unexpected EOF
	}()
	defer ln.Close()

	r := NewRemoteResolver(t.TempDir())
	// No cached body for this URL — so the read error returns the error path at line 111.
	_, err = r.fetchHTTP(context.Background(), "http://"+addr+"/config.yaml")
	if err == nil {
		t.Fatal("expected error from truncated body with no cache, got nil")
	}
	if !strings.Contains(err.Error(), "reading response body") {
		t.Errorf("expected 'reading response body' in error, got: %v", err)
	}
}

// TestLoadWithDepthAndResolver_RemoteTemplateFails covers remote.go:305-308.
// The remote config must be: (1) valid YAML so validateYAML passes, and (2) an
// invalid Go template so processTemplate fails when applied to the fetched bytes.
// Single-quoted YAML preserves {{ call .Env }} literally; yaml.Unmarshal sees a
// plain string, but text/template.Execute fails because .Env is not callable.
func TestLoadWithDepthAndResolver_RemoteTemplateFails(t *testing.T) {
	// This YAML passes yaml.Unmarshal (valid YAML) but fails processTemplate
	// because {{ call .Env }} calls .Env as a function, but it's a map.
	badTemplateValidYAML := "version: 1\nproject: '{{ call .Env }}'\ntests: []\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, badTemplateValidYAML)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	// Local config extends the remote URL.
	mainYAML := fmt.Sprintf("version: 1\nproject: local\nextends: %s\ntests:\n  - name: t\n    run: echo ok\n", srv.URL)
	mainPath := filepath.Join(tmpDir, ".smokesig.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	resolver := NewRemoteResolver(t.TempDir())
	_, err := loadWithDepthAndResolver(mainPath, 0, resolver)
	if err == nil {
		t.Fatal("expected error for invalid template in remote config, got nil")
	}
	if !strings.Contains(err.Error(), "processing template in remote config") {
		t.Errorf("expected 'processing template in remote config' in error, got: %v", err)
	}
}

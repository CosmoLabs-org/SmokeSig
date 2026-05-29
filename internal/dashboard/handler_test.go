// Test helpers testStore() and makePayload() defined in store_test.go (same package).

package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- handleResults tests ---

func TestHandleResults_PostValidPayload(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	payload := makePayload("cosmo-api", 10, 10, 0, 3400)
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["stored"] {
		t.Error("expected stored=true")
	}
}

func TestHandleResults_MethodNotAllowed(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/results", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestHandleResults_ApiKeyRequiredAndCorrect(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "secret-key")

	payload := makePayload("cosmo-api", 5, 5, 0, 100)

	t.Run("missing_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("wrong_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
		req.Header.Set("X-API-Key", "wrong")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("correct_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
		req.Header.Set("X-API-Key", "secret-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
		}
	})
}

func TestHandleResults_InvalidJSON(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleResults_MissingProjectField(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	payload := `{"total": 5, "passed": 5, "failed": 0}`
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleResults_EmptyBody(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- handleProjects tests ---

func TestHandleProjects_Empty(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	summary := resp["summary"].(map[string]interface{})
	if summary["total_projects"].(float64) != 0 {
		t.Errorf("total_projects = %v, want 0", summary["total_projects"])
	}
}

func TestHandleProjects_WithData(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	// Insert a healthy and a failing project
	store.InsertRun("proj-healthy", makePayload("proj-healthy", 10, 10, 0, 100))
	store.InsertRun("proj-failing", makePayload("proj-failing", 5, 3, 2, 200))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	summary := resp["summary"].(map[string]interface{})
	if summary["total_projects"].(float64) != 2 {
		t.Errorf("total_projects = %v, want 2", summary["total_projects"])
	}
	if summary["healthy"].(float64) != 1 {
		t.Errorf("healthy = %v, want 1", summary["healthy"])
	}
	if summary["failing"].(float64) != 1 {
		t.Errorf("failing = %v, want 1", summary["failing"])
	}
}

func TestHandleProjects_MethodNotAllowed(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// --- handleProjectHistory tests ---

func TestHandleProjectHistory_WithData(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	for i := 0; i < 5; i++ {
		store.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/cosmo-api/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["project"] != "cosmo-api" {
		t.Errorf("project = %v, want cosmo-api", resp["project"])
	}
	runs := resp["runs"].([]interface{})
	if len(runs) != 5 {
		t.Errorf("runs = %d, want 5", len(runs))
	}
}

func TestHandleProjectHistory_WithLimit(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	for i := 0; i < 10; i++ {
		store.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/cosmo-api/history?limit=3", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	runs := resp["runs"].([]interface{})
	if len(runs) != 3 {
		t.Errorf("runs = %d, want 3", len(runs))
	}
}

func TestHandleProjectHistory_TrailingSlash_NoName(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	// ServeMux registers "/api/projects/" as a subtree pattern → handleProjectHistory.
	// Requesting exactly "/api/projects/" reaches handleProjectHistory (no redirect).
	// TrimPrefix("/api/projects/", "/api/projects/") = "" → triggers the empty-name guard at line 104.
	req := httptest.NewRequest(http.MethodGet, "/api/projects/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleProjectHistory_MethodNotAllowed(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodPost, "/api/projects/cosmo-api/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleProjectHistory_NonexistentProject(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/nonexistent/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["project"] != "nonexistent" {
		t.Errorf("project = %v, want nonexistent", resp["project"])
	}
	// runs is nil (JSON null) when no records exist — not an empty array
	if resp["runs"] != nil {
		runs, ok := resp["runs"].([]interface{})
		if !ok || len(runs) != 0 {
			t.Errorf("runs = %v, want null or empty", resp["runs"])
		}
	}
}

// --- DashboardHandler tests ---

func TestDashboardHandler_ServesIndex(t *testing.T) {
	handler := DashboardHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body, _ := io.ReadAll(w.Body)
	if len(body) == 0 {
		t.Error("expected non-empty body from dashboard handler")
	}
}

// --- Error path tests (closed DB triggers 500 responses) ---

// TestHandleResults_StoreError verifies that an InsertRun failure returns 500.
func TestHandleResults_StoreError(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	// Close DB so InsertRun fails.
	s.db.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, s, "")

	payload := makePayload("cosmo-api", 5, 5, 0, 100)
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// TestHandleProjects_StoreError verifies that a GetProjects failure returns 500.
func TestHandleProjects_StoreError(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.db.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, s, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleProjectHistory_StoreError verifies that a GetProjectHistory failure returns 500.
func TestHandleProjectHistory_StoreError(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.db.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, s, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/cosmo-api/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleResults_NonObjectJSON verifies that a valid JSON non-object (array) fails the peek unmarshal.
// json.NewDecoder decodes a JSON array into RawMessage fine, but json.Unmarshal into a struct fails.
func TestHandleResults_NonObjectJSON(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	// A JSON array is valid JSON but cannot unmarshal into the peek struct {Project string}.
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(`[1,2,3]`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestHandleProjectHistory_SlashHistoryPath verifies the second empty-name guard:
// when URL.Path is "/api/projects//history", TrimPrefix yields "/history" and
// TrimSuffix("/history", "/history") yields "" → 400.
// ServeMux redirects double-slash paths (307), so we invoke the handler directly.
func TestHandleProjectHistory_SlashHistoryPath(t *testing.T) {
	store := testStore(t)
	h := &handler{store: store, apiKey: ""}

	req := httptest.NewRequest(http.MethodGet, "/api/projects//history", nil)
	// Force the raw path so strings.TrimPrefix sees "/history" after the prefix strip.
	req.URL.Path = "/api/projects//history"
	w := httptest.NewRecorder()
	h.handleProjectHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// TestHandleProjectHistory_InvalidLimit verifies that a non-numeric limit param is ignored (uses default).
func TestHandleProjectHistory_InvalidLimit(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	for i := 0; i < 5; i++ {
		store.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/cosmo-api/history?limit=notanumber", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Invalid limit falls back to default (50), all 5 runs should be returned.
	runs := resp["runs"].([]interface{})
	if len(runs) != 5 {
		t.Errorf("runs = %d, want 5", len(runs))
	}
}

// --- RegisterRoutes integration test ---

func TestRegisterRoutes_AllEndpointsRegistered(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "test-key")

	// Verify all three routes work
	endpoints := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/api/results", http.StatusForbidden}, // no key provided
		{http.MethodGet, "/api/projects", http.StatusOK},
		{http.MethodGet, "/api/projects/x/history", http.StatusOK},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != ep.want {
			t.Errorf("%s %s: status = %d, want %d", ep.method, ep.path, w.Code, ep.want)
		}
	}
}

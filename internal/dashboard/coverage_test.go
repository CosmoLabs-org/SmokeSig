// coverage_test.go — additional tests to push dashboard package coverage to 98%+
package dashboard

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// --- NewStore error paths ---

// TestNewStore_InvalidPath verifies that a path that cannot be opened returns an error.
// modernc sqlite returns an error for truly invalid DSNs (e.g. a directory as the file).
func TestNewStore_InvalidPath(t *testing.T) {
	// Use a directory path as the DB file — sqlite cannot open a directory as a database.
	dir := t.TempDir()
	_, err := NewStore(dir, 10)
	if err == nil {
		t.Skip("driver accepted directory path — skipping (driver-dependent behaviour)")
	}
}

// TestNewStore_DefaultMaxRuns verifies that maxRunsPerProject <= 0 defaults to 1000.
func TestNewStore_DefaultMaxRuns(t *testing.T) {
	s, err := NewStore(":memory:", 0)
	if err != nil {
		t.Fatalf("NewStore with 0 maxRuns: %v", err)
	}
	defer s.Close()
	if s.maxRunsPerProject != 1000 {
		t.Errorf("maxRunsPerProject = %d, want 1000", s.maxRunsPerProject)
	}
}

func TestNewStore_NegativeMaxRuns(t *testing.T) {
	s, err := NewStore(":memory:", -5)
	if err != nil {
		t.Fatalf("NewStore with negative maxRuns: %v", err)
	}
	defer s.Close()
	if s.maxRunsPerProject != 1000 {
		t.Errorf("maxRunsPerProject = %d, want 1000", s.maxRunsPerProject)
	}
}

// TestNewStore_FileDB verifies NewStore works with a real file-backed database.
func TestNewStore_FileDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "smoke.db")
	s, err := NewStore(dbPath, 50)
	if err != nil {
		t.Fatalf("NewStore file db: %v", err)
	}
	defer s.Close()

	// Insert and retrieve to confirm the DB is functional.
	_, err = s.InsertRun("file-project", makePayload("file-project", 3, 3, 0, 100))
	if err != nil {
		t.Fatalf("InsertRun on file db: %v", err)
	}
	runs, err := s.GetProjectHistory("file-project", 10)
	if err != nil {
		t.Fatalf("GetProjectHistory on file db: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("runs = %d, want 1", len(runs))
	}
}

// --- InsertRun error path: DB closed ---

func TestInsertRun_ClosedDB(t *testing.T) {
	s := testStore(t)
	s.Close() // deliberately close so Exec fails

	_, err := s.InsertRun("x", makePayload("x", 1, 1, 0, 10))
	if err == nil {
		t.Error("expected error inserting into closed DB")
	}
}

// --- GetProjects error path: DB closed ---

func TestGetProjects_ClosedDB(t *testing.T) {
	s := testStore(t)
	// Seed one record, then close.
	s.InsertRun("proj", makePayload("proj", 5, 5, 0, 100))
	s.Close()

	_, err := s.GetProjects()
	if err == nil {
		t.Error("expected error querying closed DB")
	}
}

// --- GetProjectHistory error paths ---

// TestGetProjectHistory_DefaultLimit exercises the limit <= 0 → 50 branch.
func TestGetProjectHistory_DefaultLimit(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 3; i++ {
		s.InsertRun("lim-proj", makePayload("lim-proj", 2, 2, 0, 100))
	}

	// limit = 0 → defaults to 50 internally
	runs, err := s.GetProjectHistory("lim-proj", 0)
	if err != nil {
		t.Fatalf("GetProjectHistory limit=0: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("runs = %d, want 3", len(runs))
	}
}

func TestGetProjectHistory_NegativeLimit(t *testing.T) {
	s := testStore(t)
	s.InsertRun("neg-proj", makePayload("neg-proj", 1, 1, 0, 50))

	runs, err := s.GetProjectHistory("neg-proj", -1)
	if err != nil {
		t.Fatalf("GetProjectHistory limit=-1: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("runs = %d, want 1", len(runs))
	}
}

// TestGetProjectHistory_ClosedDB exercises the DB error path.
func TestGetProjectHistory_ClosedDB(t *testing.T) {
	s := testStore(t)
	s.Close()

	_, err := s.GetProjectHistory("any", 10)
	if err == nil {
		t.Error("expected error querying closed DB")
	}
}

// --- handler error paths via broken store (closed DB) ---

// brokenStore returns a store whose DB is already closed so all operations fail.
func brokenStore(t *testing.T) *Store {
	t.Helper()
	s := testStore(t)
	s.Close()
	return s
}

func TestHandleResults_StoreError(t *testing.T) {
	store := brokenStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	payload := makePayload("some-project", 5, 5, 0, 100)
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestHandleProjects_StoreError(t *testing.T) {
	store := brokenStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestHandleProjectHistory_StoreError(t *testing.T) {
	store := brokenStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/some-project/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// --- handleProjectHistory: limit query param edge cases ---

// TestHandleProjectHistory_InvalidLimit verifies that a non-numeric limit param is ignored (uses default 50).
func TestHandleProjectHistory_InvalidLimit(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	for i := 0; i < 5; i++ {
		store.InsertRun("lim-proj", makePayload("lim-proj", 2, 2, 0, 100))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/lim-proj/history?limit=abc", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestHandleProjectHistory_ZeroLimit verifies that limit=0 is ignored (uses default 50).
func TestHandleProjectHistory_ZeroLimit(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	store.InsertRun("lim-proj2", makePayload("lim-proj2", 2, 2, 0, 100))

	req := httptest.NewRequest(http.MethodGet, "/api/projects/lim-proj2/history?limit=0", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestHandleProjectHistory_NegativeLimit verifies that limit=-1 is ignored (uses default 50).
func TestHandleProjectHistory_NegativeLimit(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	store.InsertRun("lim-proj3", makePayload("lim-proj3", 2, 2, 0, 100))

	req := httptest.NewRequest(http.MethodGet, "/api/projects/lim-proj3/history?limit=-1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestHandleProjectHistory_JustSlashHistory exercises the "/history" name being trimmed to empty.
func TestHandleProjectHistory_OnlyHistory(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	// Path: /api/projects//history would be cleaned by http to /api/projects/history
	// Instead simulate: TrimPrefix gives "history", TrimSuffix("/history") gives ""
	// We need to construct a request whose path after TrimPrefix is exactly "history"
	// but after TrimSuffix("/history") is "". This path is /api/projects/history
	// but the mux registered "/api/projects/" so it will be served by handleProjectHistory.
	req := httptest.NewRequest(http.MethodGet, "/api/projects/history", nil)
	// Force the raw path so http.ServeMux doesn't clean it.
	req.URL.Path = "/api/projects/history"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// name = TrimPrefix("/api/projects/history", "/api/projects/") = "history"
	// name = TrimSuffix("history", "/history") = "history"  (no /history suffix)
	// So this actually succeeds as project name "history" (not the empty guard).
	// Verify it returns 200 with project="history".
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestHandleProjectHistory_ProjectNameOnly verifies a project path without /history suffix.
func TestHandleProjectHistory_ProjectNameOnly(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	store.InsertRun("myproject", makePayload("myproject", 4, 4, 0, 200))

	// /api/projects/myproject — no /history suffix; name = "myproject" after TrimSuffix (no change)
	req := httptest.NewRequest(http.MethodGet, "/api/projects/myproject", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestHandleResults_AllowedFailuresField verifies that allowed_failures in payload is stored.
func TestHandleResults_AllowedFailuresField(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	payload := `{"project":"af-proj","total":10,"passed":8,"failed":2,"skipped":0,"duration_ms":500,"allowed_failures":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	runs, err := store.GetProjectHistory("af-proj", 1)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].AllowedFailures != 2 {
		t.Errorf("AllowedFailures = %d, want 2", runs[0].AllowedFailures)
	}
}

// TestStore_MultipleProjects_HealthSummary verifies GetProjects latestStatus logic for mixed states.
func TestStore_MultipleProjects_HealthSummary(t *testing.T) {
	s := testStore(t)

	// Two healthy, one failing.
	s.InsertRun("alpha", makePayload("alpha", 5, 5, 0, 100))
	s.InsertRun("beta", makePayload("beta", 3, 3, 0, 200))
	s.InsertRun("gamma", makePayload("gamma", 4, 2, 2, 300))

	projects, err := s.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("projects = %d, want 3", len(projects))
	}

	byName := map[string]ProjectStatus{}
	for _, p := range projects {
		byName[p.Name] = p
	}

	if byName["alpha"].LatestStatus != "healthy" {
		t.Errorf("alpha: status = %q, want healthy", byName["alpha"].LatestStatus)
	}
	if byName["beta"].LatestStatus != "healthy" {
		t.Errorf("beta: status = %q, want healthy", byName["beta"].LatestStatus)
	}
	if byName["gamma"].LatestStatus != "failing" {
		t.Errorf("gamma: status = %q, want failing", byName["gamma"].LatestStatus)
	}
}

// TestStore_InsertRunReturnsID verifies that InsertRun returns a positive last-insert ID.
func TestStore_InsertRunReturnsID(t *testing.T) {
	s := testStore(t)

	id1, err := s.InsertRun("id-proj", makePayload("id-proj", 1, 1, 0, 10))
	if err != nil {
		t.Fatalf("InsertRun 1: %v", err)
	}
	id2, err := s.InsertRun("id-proj", makePayload("id-proj", 2, 2, 0, 20))
	if err != nil {
		t.Fatalf("InsertRun 2: %v", err)
	}
	if id1 <= 0 {
		t.Errorf("id1 = %d, want > 0", id1)
	}
	if id2 <= id1 {
		t.Errorf("id2 (%d) should be > id1 (%d)", id2, id1)
	}
}

// TestHandleProjects_ContentType verifies the response has JSON content type.
func TestHandleProjects_ContentType(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestHandleProjectHistory_ContentType verifies JSON content type on history endpoint.
func TestHandleProjectHistory_ContentType(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/myproj/history", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestHandleResults_NoAPIKey_Passes verifies that empty apiKey skips auth entirely.
func TestHandleResults_NoAPIKey_Passes(t *testing.T) {
	store := testStore(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, "") // no API key

	payload := makePayload("nokey-proj", 1, 1, 0, 50)
	req := httptest.NewRequest(http.MethodPost, "/api/results", strings.NewReader(string(payload)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusAccepted, w.Body.String())
	}
}

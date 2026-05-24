package dashboard

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// RegisterRoutes adds dashboard API routes to mux.
func RegisterRoutes(mux *http.ServeMux, store *Store, apiKey string) {
	h := &handler{store: store, apiKey: apiKey}
	mux.HandleFunc("/api/results", h.handleResults)
	mux.HandleFunc("/api/projects", h.handleProjects)
	mux.HandleFunc("/api/projects/", h.handleProjectHistory)
}

type handler struct {
	store  *Store
	apiKey string
}

func (h *handler) handleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if h.apiKey != "" {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(h.apiKey)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusForbidden)
			return
		}
	}

	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	var peek struct {
		Project string `json:"project"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if peek.Project == "" {
		http.Error(w, `{"error":"project field required"}`, http.StatusBadRequest)
		return
	}

	if _, err := h.store.InsertRun(peek.Project, raw); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]bool{"stored": true})
}

func (h *handler) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	projects, err := h.store.GetProjects()
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	healthy, failing := 0, 0
	for _, p := range projects {
		if p.LatestStatus == "healthy" {
			healthy++
		} else {
			failing++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"projects": projects,
		"summary": map[string]interface{}{
			"total_projects": len(projects),
			"healthy":        healthy,
			"failing":        failing,
		},
	})
}

func (h *handler) handleProjectHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// /api/projects/{name}/history
	name := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	if name == "" {
		http.Error(w, `{"error":"project name required"}`, http.StatusBadRequest)
		return
	}
	name = strings.TrimSuffix(name, "/history")
	if name == "" {
		http.Error(w, `{"error":"project name required"}`, http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	runs, err := h.store.GetProjectHistory(name, limit)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project": name,
		"runs":    runs,
	})
}

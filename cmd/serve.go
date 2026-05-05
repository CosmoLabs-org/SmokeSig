package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/dashboard"
	"github.com/CosmoLabs-org/SmokeSig/internal/reporter"
	"github.com/CosmoLabs-org/SmokeSig/internal/runner"
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP health endpoint for container probes",
	Long:  "Expose a /healthz endpoint that runs smoke tests on each request",
	RunE:  runServe,
}

var (
	servePort       string
	servePath       string
	serveConfigFile string
	serveDashboard  bool
	serveAPIKey     string
	serveDBPath     string
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&servePort, "port", "p", "8080", "Port to listen on")
	serveCmd.Flags().StringVar(&servePath, "path", "/healthz", "Health endpoint path")
	serveCmd.Flags().StringVarP(&serveConfigFile, "file", "f", ".smokesig.yaml", "Config file path")
	serveCmd.Flags().BoolVar(&serveDashboard, "dashboard", false, "Enable dashboard aggregation mode")
	serveCmd.Flags().StringVar(&serveAPIKey, "api-key", "", "API key for POST /api/results (X-API-Key header)")
	serveCmd.Flags().StringVar(&serveDBPath, "db-path", "smoke-dashboard.db", "SQLite database path")
}

// noopReporter satisfies reporter.Reporter without emitting any output.
type noopReporter struct{}

func newNoopReporter() *noopReporter { return &noopReporter{} }

func (n *noopReporter) PrereqStart(_ string)                              {}
func (n *noopReporter) PrereqResult(_ reporter.PrereqResultData)          {}
func (n *noopReporter) TestStart(_ string)                                {}
func (n *noopReporter) TestResult(_ reporter.TestResultData)              {}
func (n *noopReporter) Summary(_ reporter.SuiteResultData)                {}

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status     string      `json:"status"`
	Tests      testCounts  `json:"tests"`
	DurationMs int64       `json:"duration_ms"`
}

type testCounts struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// buildHandler returns an http.HandlerFunc that runs smoke tests on every
// request and responds with the appropriate status and JSON body.
func buildHandler(configFile string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		cfg, err := schema.Load(configFile)
		if err != nil {
			writeHealthError(w, http.StatusInternalServerError, fmt.Sprintf("load config: %v", err))
			return
		}
		if err := schema.Validate(cfg); err != nil {
			writeHealthError(w, http.StatusInternalServerError, fmt.Sprintf("invalid config: %v", err))
			return
		}

		configDir := filepath.Dir(configFile)
		if !filepath.IsAbs(configDir) {
			cwd, _ := os.Getwd()
			configDir = filepath.Join(cwd, configDir)
		}

		noop := newNoopReporter()
		rn := &runner.Runner{
			Config:    cfg,
			Reporter:  noop,
			ConfigDir: configDir,
		}

		result, err := rn.Run(runner.RunOptions{})
		elapsed := time.Since(start)
		if err != nil {
			writeHealthError(w, http.StatusInternalServerError, fmt.Sprintf("run: %v", err))
			return
		}

		status := "healthy"
		httpStatus := http.StatusOK
		if result.Failed > 0 {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}

		resp := healthResponse{
			Status: status,
			Tests: testCounts{
				Total:  result.Total,
				Passed: result.Passed,
				Failed: result.Failed,
			},
			DurationMs: elapsed.Milliseconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp) //nolint:errcheck
	}
}

func writeHealthError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func runServe(cmd *cobra.Command, args []string) error {
	addr := ":" + servePort

	mux := http.NewServeMux()
	mux.HandleFunc(servePath, buildHandler(serveConfigFile))

	if serveDashboard {
		store, err := dashboard.NewStore(serveDBPath, 1000)
		if err != nil {
			return fmt.Errorf("open dashboard database: %w", err)
		}
		defer store.Close()
		dashboard.RegisterRoutes(mux, store, serveAPIKey)
		mux.Handle("/dashboard", dashboard.DashboardHandler())
		fmt.Fprintf(os.Stderr, "Dashboard mode: %s/api/results\n", "http://localhost"+addr)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx) //nolint:errcheck
	}()

	fmt.Fprintf(os.Stderr, "smoke serve listening on %s%s\n", addr, servePath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

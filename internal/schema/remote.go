package schema

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type RemoteResolver struct {
	cacheDir string
	client   *http.Client
}

func NewRemoteResolver(cacheDir string) *RemoteResolver {
	if cacheDir == "" {
		homeDir, err := os.UserCacheDir()
		if err != nil {
			homeDir = os.TempDir()
		}
		cacheDir = filepath.Join(homeDir, "cosmo-smoke")
	}
	return &RemoteResolver{
		cacheDir: cacheDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (r *RemoteResolver) Resolve(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		if u.Scheme == "http" {
			log.Printf("[WARN] fetching from HTTP (non-HTTPS) URL: %s", rawURL)
		}
		return r.fetchHTTP(ctx, rawURL)
	case "file":
		return r.fetchFile(u.Path)
	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s (supported: http, https, file)", u.Scheme)
	}
}

func (r *RemoteResolver) fetchHTTP(ctx context.Context, rawURL string) ([]byte, error) {
	if err := os.MkdirAll(r.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	cacheFile := r.cachePath(rawURL)
	cachedBody, cachedETag, cachedLastMod, err := r.readCache(cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if cachedETag != "" {
		req.Header.Set("If-None-Match", cachedETag)
	}
	if cachedLastMod != "" {
		req.Header.Set("If-Modified-Since", cachedLastMod)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		if cachedBody != nil {
			log.Printf("[INFO] network error, using cached response: %v", err)
			return cachedBody, nil
		}
		return nil, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		log.Printf("[DEBUG] cache hit for %s (304 Not Modified)", rawURL)
		return cachedBody, nil
	}

	if resp.StatusCode != http.StatusOK {
		if cachedBody != nil {
			log.Printf("[WARN] HTTP %d for %s, using cached response", resp.StatusCode, rawURL)
			return cachedBody, nil
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB max
	if err != nil {
		if cachedBody != nil {
			log.Printf("[WARN] error reading response, using cached: %v", err)
			return cachedBody, nil
		}
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if err := r.validateYAML(body); err != nil {
		return nil, fmt.Errorf("invalid YAML from remote: %w", err)
	}

	etag := resp.Header.Get("ETag")
	lastMod := resp.Header.Get("Last-Modified")
	if err := r.writeCache(cacheFile, body, etag, lastMod); err != nil {
		log.Printf("[WARN] failed to write cache: %v", err)
	}

	return body, nil
}

func (r *RemoteResolver) fetchFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if err := r.validateYAML(data); err != nil {
		return nil, fmt.Errorf("invalid YAML in file: %w", err)
	}

	return data, nil
}

func (r *RemoteResolver) validateYAML(data []byte) error {
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return nil
}

func (r *RemoteResolver) cachePath(url string) string {
	h := sha256.Sum256([]byte(url))
	filename := hex.EncodeToString(h[:]) + ".yaml"
	return filepath.Join(r.cacheDir, filename)
}

type cacheEntry struct {
	ETag         string `yaml:"etag"`
	LastModified string `yaml:"last_modified"`
	Body         string `yaml:"body"`
}

func (r *RemoteResolver) readCache(path string) (body []byte, etag string, lastMod string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", err
	}

	var entry cacheEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return nil, "", "", fmt.Errorf("unmarshaling cache entry: %w", err)
	}

	return []byte(entry.Body), entry.ETag, entry.LastModified, nil
}

func (r *RemoteResolver) writeCache(path string, body []byte, etag, lastMod string) error {
	entry := cacheEntry{
		ETag:         etag,
		LastModified: lastMod,
		Body:         string(body),
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling cache entry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

func MergeConfigs(base, overlay SmokeConfig) SmokeConfig {
	merged := base
	merged.Tests = append([]Test{}, base.Tests...)
	merged.Prereqs = append([]Prerequisite{}, base.Prereqs...)

	if overlay.Project != "" {
		merged.Project = overlay.Project
	}

	if overlay.Description != "" {
		merged.Description = overlay.Description
	}

	if overlay.Settings.Timeout.Duration > 0 {
		merged.Settings.Timeout = overlay.Settings.Timeout
	}

	merged.Settings.FailFast = overlay.Settings.FailFast
	merged.Settings.Parallel = overlay.Settings.Parallel
	merged.Settings.Monorepo = overlay.Settings.Monorepo

	if len(overlay.Settings.MonorepoExclude) > 0 {
		merged.Settings.MonorepoExclude = overlay.Settings.MonorepoExclude
	}

	merged.OTel.Enabled = overlay.OTel.Enabled

	if overlay.OTel.JaegerURL != "" {
		merged.OTel.JaegerURL = overlay.OTel.JaegerURL
	}

	if overlay.OTel.ServiceName != "" {
		merged.OTel.ServiceName = overlay.OTel.ServiceName
	}

	merged.OTel.TracePropagation = overlay.OTel.TracePropagation

	if overlay.OTel.ExportURL != "" {
		merged.OTel.ExportURL = overlay.OTel.ExportURL
	}

	if len(overlay.OTel.ExportHeaders) > 0 {
		merged.OTel.ExportHeaders = overlay.OTel.ExportHeaders
	}

	if len(overlay.Includes) > 0 {
		merged.Includes = overlay.Includes
	}

	if len(overlay.Prereqs) > 0 {
		merged.Prereqs = append(merged.Prereqs, overlay.Prereqs...)
	}

	if len(overlay.Tests) > 0 {
		merged.Tests = append(merged.Tests, overlay.Tests...)
	}

	return merged
}

func LoadWithResolver(path string, resolver *RemoteResolver) (*SmokeConfig, error) {
	return loadWithDepthAndResolver(path, 0, resolver)
}

func loadWithDepthAndResolver(path string, depth int, resolver *RemoteResolver) (*SmokeConfig, error) {
	if depth > 10 {
		return nil, fmt.Errorf("include depth exceeded (max 10): circular includes?")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	processed, err := processTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("processing template: %w", err)
	}

	cfg, err := Parse(processed)
	if err != nil {
		return nil, err
	}

	configDir := filepath.Dir(path)

	for _, inc := range cfg.Includes {
		incPath := inc
		if !filepath.IsAbs(inc) {
			incPath = filepath.Join(configDir, inc)
		}

		incCfg, err := loadWithDepthAndResolver(incPath, depth+1, resolver)
		if err != nil {
			return nil, fmt.Errorf("loading include %q: %w", inc, err)
		}

		cfg.Prereqs = append(incCfg.Prereqs, cfg.Prereqs...)
		cfg.Tests = append(incCfg.Tests, cfg.Tests...)
	}

	if cfg.Extends != "" {
		if resolver == nil {
			resolver = NewRemoteResolver("")
		}

		ctx := context.Background()
		remoteData, err := resolver.Resolve(ctx, cfg.Extends)
		if err != nil {
			return nil, fmt.Errorf("resolving extends %q: %w", cfg.Extends, err)
		}

		processed, err := processTemplate(remoteData)
		if err != nil {
			return nil, fmt.Errorf("processing template in remote config: %w", err)
		}

		remoteCfg, err := Parse(processed)
		if err != nil {
			return nil, fmt.Errorf("parsing remote config: %w", err)
		}

		merged := MergeConfigs(*remoteCfg, *cfg)
		merged.Extends = ""
		cfg = &merged
	}

	return cfg, nil
}

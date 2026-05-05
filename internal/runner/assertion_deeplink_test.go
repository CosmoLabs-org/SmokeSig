package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckAssetlinks_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/assetlinks.json" {
			w.WriteHeader(200)
			fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.myapp"}}]`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	result := CheckAssetlinks(srv.URL, "com.myapp")
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckAssetlinks_MissingPackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.other"}}]`)
	}))
	defer srv.Close()

	result := CheckAssetlinks(srv.URL, "com.myapp")
	if result.Passed {
		t.Error("expected failure for mismatched package")
	}
}

func TestCheckAssetlinks_HTTPError(t *testing.T) {
	result := CheckAssetlinks("http://127.0.0.1:1", "com.myapp")
	if result.Passed {
		t.Error("expected failure for unreachable host")
	}
}

func TestCheckAssetlinks_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	result := CheckAssetlinks(srv.URL, "com.myapp")
	if result.Passed {
		t.Error("expected failure for HTTP 404")
	}
}

func TestCheckAASA_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apple-app-site-association") {
			w.WriteHeader(200)
			fmt.Fprintln(w, `{"applinks":{"details":[{"appIDs":["ABCDE12345.com.myapp"],"components":[{"/*":{}}]}]}}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	result := CheckAASA(srv.URL, "com.myapp")
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Actual)
	}
}

func TestCheckAASA_MissingBundleID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apple-app-site-association") {
			w.WriteHeader(200)
			fmt.Fprintln(w, `{"applinks":{"details":[{"appIDs":["ABCDE12345.com.other"]}]}}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	result := CheckAASA(srv.URL, "com.myapp")
	if result.Passed {
		t.Error("expected failure for mismatched bundle ID")
	}
}

func TestCheckAASA_NoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	result := CheckAASA(srv.URL, "com.myapp")
	if result.Passed {
		t.Error("expected failure when no AASA file")
	}
}

func TestCheckDeepLink_HTTPSRunsAssetlinksAndAASA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/assetlinks.json" {
			w.WriteHeader(200)
			fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.myapp"}}]`)
			return
		}
		if strings.Contains(r.URL.Path, "apple-app-site-association") {
			w.WriteHeader(200)
			fmt.Fprintln(w, `{"applinks":{"details":[{"appIDs":["com.myapp"]}]}}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	dl := &schema.DeepLinkCheck{URL: srv.URL + "/product/123", AndroidPackage: "com.myapp", Tier: "config-only"}
	results := CheckDeepLink(dl, "")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 assertions, got %d", len(results))
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("expected all tier 1 checks to pass, got failure: %s = %s", r.Type, r.Actual)
		}
	}
}

func TestCheckDeepLink_CustomSchemeSkipsHTTP(t *testing.T) {
	dl := &schema.DeepLinkCheck{URL: "myapp://test", Tier: "config-only"}
	results := CheckDeepLink(dl, "")
	for _, r := range results {
		if r.Type == "deep_link.assetlinks" || r.Type == "deep_link.aasa" {
			t.Error("custom scheme should not trigger HTTP checks")
		}
	}
}

func TestCheckDeepLink_FullResolveNoTools(t *testing.T) {
	if hasTool("adb") || hasTool("xcrun") {
		t.Skip("skipping: adb or xcrun available on this machine")
	}
	dl := &schema.DeepLinkCheck{URL: "https://example.com/path", Tier: "full-resolve"}
	results := CheckDeepLink(dl, "")
	found := false
	for _, r := range results {
		if r.Type == "deep_link.resolve" && !r.Passed {
			found = true
		}
	}
	if !found {
		t.Error("expected resolve failure when tier=full-resolve and no tools available")
	}
}

func TestCheckDeepLink_DisableAssetlinks(t *testing.T) {
	dl := &schema.DeepLinkCheck{URL: "https://example.com/path", CheckAssetlinks: boolPtr(false), Tier: "config-only"}
	results := CheckDeepLink(dl, "")
	for _, r := range results {
		if r.Type == "deep_link.assetlinks" {
			t.Error("assetlinks check should be skipped when disabled")
		}
	}
}

func boolPtr(b bool) *bool { return &b }

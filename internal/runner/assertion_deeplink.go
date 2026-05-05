package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

type assetlinkStatement struct {
	Relation []string `json:"relation"`
	Target   struct {
		Namespace   string `json:"namespace"`
		PackageName string `json:"package_name"`
	} `json:"target"`
}

// CheckAssetlinks fetches and validates .well-known/assetlinks.json for Android App Links.
func CheckAssetlinks(baseURL, expectedPackage string) AssertionResult {
	url := strings.TrimRight(baseURL, "/") + "/.well-known/assetlinks.json"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "assetlinks.json accessible", Actual: fmt.Sprintf("HTTP error: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "HTTP 200", Actual: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	var statements []assetlinkStatement
	if err := json.NewDecoder(resp.Body).Decode(&statements); err != nil {
		return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "valid JSON", Actual: fmt.Sprintf("parse error: %v", err)}
	}
	for _, s := range statements {
		hasRelation := false
		for _, r := range s.Relation {
			if r == "delegate_permission/common.handle_all_urls" {
				hasRelation = true
				break
			}
		}
		if hasRelation && s.Target.Namespace == "android_app" {
			if expectedPackage == "" || s.Target.PackageName == expectedPackage {
				return AssertionResult{Type: "deep_link.assetlinks", Passed: true}
			}
		}
	}
	return AssertionResult{
		Type: "deep_link.assetlinks", Passed: false,
		Expected: fmt.Sprintf("package %q in assetlinks.json", expectedPackage),
		Actual:   "no matching statement found",
	}
}

// CheckAASA fetches and validates apple-app-site-association for iOS Universal Links.
func CheckAASA(baseURL, expectedBundleID string) AssertionResult {
	for _, path := range []string{"/apple-app-site-association", "/.well-known/apple-app-site-association"} {
		url := strings.TrimRight(baseURL, "/") + path
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		var aasa struct {
			Applinks struct {
				Details []struct {
					AppIDs []string `json:"appIDs"`
				} `json:"details"`
			} `json:"applinks"`
		}
		err = json.NewDecoder(resp.Body).Decode(&aasa)
		resp.Body.Close()
		if err != nil {
			continue
		}
		for _, d := range aasa.Applinks.Details {
			for _, id := range d.AppIDs {
				if expectedBundleID == "" || strings.HasSuffix(id, expectedBundleID) {
					return AssertionResult{Type: "deep_link.aasa", Passed: true}
				}
			}
		}
	}
	return AssertionResult{
		Type: "deep_link.aasa", Passed: false,
		Expected: "valid AASA with matching bundle ID",
		Actual:   "no matching apple-app-site-association found",
	}
}

// CheckDeepLink runs the appropriate deep link checks based on URL scheme and tier setting.
func CheckDeepLink(cfg *schema.DeepLinkCheck, configDir string) []AssertionResult {
	var results []AssertionResult
	tier := cfg.Tier
	if tier == "" {
		tier = "auto"
	}

	u, _ := url.Parse(cfg.URL)
	isWebURL := u.Scheme == "http" || u.Scheme == "https"

	// Tier 1: HTTP checks for web URLs (http/https)
	if isWebURL {
		baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		if cfg.CheckAssetlinks == nil || *cfg.CheckAssetlinks {
			results = append(results, CheckAssetlinks(baseURL, cfg.AndroidPackage))
		}
		if cfg.CheckAASA == nil || *cfg.CheckAASA {
			results = append(results, CheckAASA(baseURL, cfg.IOSBundleID))
		}
	}

	// Tier 2: Tool-augmented resolution
	if tier == "full-resolve" || tier == "auto" {
		adbAvailable := hasTool("adb")
		xcrunAvailable := hasTool("xcrun")
		if adbAvailable || xcrunAvailable {
			results = append(results, resolveDeepLink(cfg, adbAvailable, xcrunAvailable)...)
		} else if tier == "full-resolve" {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve", Passed: false,
				Expected: "adb or xcrun available",
				Actual:   "no mobile resolution tools found",
			})
		}
	}

	return results
}

// hasTool checks if a CLI tool is available on PATH.
func hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// resolveDeepLink runs platform-specific resolution tools (Tier 2).
func resolveDeepLink(cfg *schema.DeepLinkCheck, adb, xcrun bool) []AssertionResult {
	var results []AssertionResult
	if adb {
		out, err := exec.Command("adb", "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", cfg.URL).CombinedOutput()
		output := string(out)
		if err != nil {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve.android", Passed: false,
				Expected: "URL resolves via adb",
				Actual:   fmt.Sprintf("adb error: %v: %s", err, output),
			})
		} else if strings.Contains(output, "Error") || strings.Contains(output, "ActivityNotFoundException") {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve.android", Passed: false,
				Expected: "URL resolves via adb",
				Actual:   fmt.Sprintf("resolution failed: %s", output),
			})
		} else {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve.android", Passed: true,
			})
		}
	}
	if xcrun {
		out, err := exec.Command("xcrun", "simctl", "openurl", "booted", cfg.URL).CombinedOutput()
		if err != nil {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve.ios", Passed: false,
				Expected: "URL accepted by iOS simulator",
				Actual:   fmt.Sprintf("xcrun error: %v: %s", err, string(out)),
			})
		} else {
			results = append(results, AssertionResult{
				Type: "deep_link.resolve.ios", Passed: true,
			})
		}
	}
	return results
}

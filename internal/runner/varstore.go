package runner

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"github.com/tidwall/gjson"
)

// VarStore holds extracted values from test assertions for use in subsequent tests.
type VarStore struct {
	mu   sync.RWMutex
	vars map[string]string
}

// NewVarStore creates an empty variable store.
func NewVarStore() *VarStore {
	return &VarStore{vars: make(map[string]string)}
}

func (v *VarStore) Set(key, value string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.vars[key] = value
}

func (v *VarStore) Get(key string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	val, ok := v.vars[key]
	return val, ok
}

// ResolveTemplate resolves {{ .Vars.X }} references in the input string.
func (v *VarStore) ResolveTemplate(input string) (string, error) {
	tmpl, err := template.New("resolve").Option("missingkey=error").Parse(input)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	var buf strings.Builder
	err = tmpl.Execute(&buf, map[string]any{"Vars": v.vars})
	if err != nil {
		return "", fmt.Errorf("resolving template: %w", err)
	}
	return buf.String(), nil
}

// IsSecret returns true if the variable name looks like it contains sensitive data.
func (v *VarStore) IsSecret(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range []string{"token", "key", "secret", "password", "auth"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// Mask replaces sensitive variable values in the input with ***REDACTED***.
func (v *VarStore) Mask(input string) string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := input
	for key, val := range v.vars {
		if v.IsSecret(key) && val != "" {
			result = strings.ReplaceAll(result, val, "***REDACTED***")
		}
	}
	return result
}

// extractedValue holds a key-value pair extracted from a test assertion.
type extractedValue struct {
	key   string
	value string
}

// extractFromJSON extracts a value from stdout using gjson path.
func extractFromJSON(stdout, path, varName string) extractedValue {
	result := gjson.Get(stdout, path)
	return extractedValue{key: varName, value: result.String()}
}

// extractFromRegex extracts a value from stdout using a regex pattern.
// Uses first capture group if available, otherwise the full match.
func extractFromRegex(stdout, pattern, varName string) extractedValue {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return extractedValue{key: varName, value: ""}
	}
	matches := re.FindStringSubmatch(stdout)
	if len(matches) >= 2 {
		return extractedValue{key: varName, value: matches[1]}
	}
	match := re.FindString(stdout)
	return extractedValue{key: varName, value: match}
}

// processExtracts runs through a test's assertions, extracts values, and stores them.
func processExtracts(expect *schema.Expect, stdout string, store *VarStore) {
	if expect.JSONField != nil && expect.JSONField.Extract != "" {
		ev := extractFromJSON(stdout, expect.JSONField.Path, expect.JSONField.Extract)
		if ev.value != "" {
			store.Set(ev.key, ev.value)
		}
	}
	if expect.StdoutMatches != "" && expect.Extract != "" {
		ev := extractFromRegex(stdout, expect.StdoutMatches, expect.Extract)
		if ev.value != "" {
			store.Set(ev.key, ev.value)
		}
	}
}

// detectChains identifies groups of tests that form dependency chains via extract/vars.
// Returns a list of chain groups, where each group is a slice of test indices that must run sequentially.
func detectChains(tests []schema.Test) [][]int {
	// Map: varName -> index of test that extracts it
	extractors := make(map[string]int)
	for i, t := range tests {
		if t.Expect.JSONField != nil && t.Expect.JSONField.Extract != "" {
			extractors[t.Expect.JSONField.Extract] = i
		}
		if t.Expect.Extract != "" {
			extractors[t.Expect.Extract] = i
		}
	}

	// Map: index -> set of indices it depends on
	deps := make(map[int]map[int]bool)
	for i, t := range tests {
		varsReferenced := findVarReferences(t)
		for v := range varsReferenced {
			if srcIdx, ok := extractors[v]; ok && srcIdx != i {
				if deps[i] == nil {
					deps[i] = make(map[int]bool)
				}
				deps[i][srcIdx] = true
			}
		}
	}

	// Group chains: find connected components in the dependency graph
	visited := make(map[int]bool)
	var groups [][]int

	for i := range tests {
		if visited[i] {
			continue
		}
		if len(deps[i]) == 0 {
			continue // independent test, skip
		}
		// BFS to find all connected tests
		group := map[int]bool{}
		queue := []int{i}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if group[cur] {
				continue
			}
			group[cur] = true
			visited[cur] = true
			// Add dependencies
			for dep := range deps[cur] {
				if !group[dep] {
					queue = append(queue, dep)
				}
			}
			// Add dependents
			for j, jDeps := range deps {
				if jDeps[cur] && !group[j] {
					queue = append(queue, j)
				}
			}
		}
		if len(group) > 1 {
			var indices []int
			for idx := range group {
				indices = append(indices, idx)
			}
			// Sort indices to maintain order
			for a := 0; a < len(indices); a++ {
				for b := a + 1; b < len(indices); b++ {
					if indices[a] > indices[b] {
						indices[a], indices[b] = indices[b], indices[a]
					}
				}
			}
			groups = append(groups, indices)
		}
	}

	return groups
}

// varRefRegex matches {{ .Vars.X }} or {{.Vars.X}} patterns.
var varRefRegex = regexp.MustCompile(`\{\{\s*\.Vars\.(\w+)\s*\}\}`)

// findVarReferences returns the set of variable names referenced in a test.
func findVarReferences(t schema.Test) map[string]bool {
	refs := make(map[string]bool)
	collectRefs(t.Run, refs)
	if t.Expect.StdoutContains != "" {
		collectRefs(t.Expect.StdoutContains, refs)
	}
	if t.Expect.StdoutMatches != "" {
		collectRefs(t.Expect.StdoutMatches, refs)
	}
	if t.Expect.StderrContains != "" {
		collectRefs(t.Expect.StderrContains, refs)
	}
	if t.Expect.StderrMatches != "" {
		collectRefs(t.Expect.StderrMatches, refs)
	}
	if t.Expect.HTTP != nil {
		collectRefs(t.Expect.HTTP.URL, refs)
		for _, h := range t.Expect.HTTP.HeaderContains {
			collectRefs(h, refs)
		}
	}
	return refs
}

func collectRefs(s string, refs map[string]bool) {
	matches := varRefRegex.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		refs[m[1]] = true
	}
}

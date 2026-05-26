package observer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/CosmoLabs-org/SmokeSig/internal/detector"
)

// StackHints provides detector-derived defaults for a project's expected ports and probe paths.
type StackHints struct {
	ExpectedPorts   []int
	ExtraProbePaths []string
}

// HintsFromDir reads portless.json and detects the project type to build merged StackHints.
func HintsFromDir(dir string) StackHints {
	var hints StackHints

	// Read portless port first — it takes priority.
	portlessPort := readPortlessPort(dir)
	if portlessPort > 0 {
		hints.ExpectedPorts = append(hints.ExpectedPorts, portlessPort)
	}

	// Detect project types and merge stack defaults.
	types := detector.Detect(dir)
	seenPorts := make(map[int]bool)
	for _, p := range hints.ExpectedPorts {
		seenPorts[p] = true
	}
	seenPaths := make(map[string]bool)

	for _, pt := range types {
		sh, ok := stackHints[pt]
		if !ok {
			continue
		}
		for _, p := range sh.ExpectedPorts {
			if !seenPorts[p] {
				seenPorts[p] = true
				hints.ExpectedPorts = append(hints.ExpectedPorts, p)
			}
		}
		for _, p := range sh.ExtraProbePaths {
			if !seenPaths[p] {
				seenPaths[p] = true
				hints.ExtraProbePaths = append(hints.ExtraProbePaths, p)
			}
		}
	}

	return hints
}

// readPortlessPort reads portless.json from dir and returns the port value, or 0.
func readPortlessPort(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, "portless.json"))
	if err != nil {
		return 0
	}
	var raw struct {
		Port int `json:"port"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0
	}
	return raw.Port
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

var stackHints = map[detector.ProjectType]StackHints{
	detector.Go:          {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/metrics", "/debug/pprof"}},
	detector.Node:        {ExpectedPorts: []int{3000}, ExtraProbePaths: []string{"/api", "/graphql"}},
	detector.Python:      {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/admin", "/api"}},
	detector.ReactNative: {ExpectedPorts: []int{8081}, ExtraProbePaths: []string{"/status"}},
	detector.Rust:        {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/api"}},
	detector.Java:        {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/actuator/health"}},
	detector.JavaGradle:  {ExpectedPorts: []int{8080}, ExtraProbePaths: []string{"/actuator/health"}},
	detector.DotNet:      {ExpectedPorts: []int{5000}, ExtraProbePaths: []string{"/swagger"}},
	detector.Ruby:        {ExpectedPorts: []int{3000}, ExtraProbePaths: []string{"/api"}},
	detector.PHP:         {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/api"}},
	detector.Elixir:      {ExpectedPorts: []int{4000}, ExtraProbePaths: []string{"/api"}},
	detector.Deno:        {ExpectedPorts: []int{8000}, ExtraProbePaths: []string{"/api"}},
	detector.Hugo:        {ExpectedPorts: []int{1313}, ExtraProbePaths: []string{"/"}},
	detector.Astro:       {ExpectedPorts: []int{4321}, ExtraProbePaths: []string{"/"}},
	detector.Jekyll:      {ExpectedPorts: []int{4000}, ExtraProbePaths: []string{"/"}},
}

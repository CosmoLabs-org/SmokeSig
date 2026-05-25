package observer

import "time"

type PortBinding struct {
	Port     int
	Protocol string
	Host     string
}

type FileSnapshot struct {
	Path string
	Size int64
	Hash string
}

type HTTPProbeResult struct {
	URL        string
	StatusCode int
	Reachable  bool
}

type Observation struct {
	Command    string
	ExitCode   int
	Stdout     string
	Stderr     string
	Ports      []PortBinding
	NewFiles   []FileSnapshot
	HTTPProbes []HTTPProbeResult
	Duration   time.Duration
}

type ObserveOptions struct {
	Command string
	Dir     string
	Timeout time.Duration
	Quiet   bool
	Output  string
}

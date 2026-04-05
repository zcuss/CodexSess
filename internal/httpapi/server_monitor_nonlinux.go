//go:build !linux && !windows

package httpapi

import (
	"os"
	"path/filepath"
	"time"
)

func collectHostMonitorSample(dataDir string, startedAt time.Time) hostMonitorSample {
	out := hostMonitorSample{
		ProcessUptimeSec: time.Since(startedAt).Seconds(),
	}
	if h, err := os.Hostname(); err == nil {
		out.Hostname = h
	}
	if stringsTrim(dataDir) == "" {
		dataDir = "."
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	return out
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\t' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

package httpapi

import (
	"net/http"
	"os"
	"runtime"
	"time"
)

type hostMonitorSample struct {
	Hostname          string
	ProcessCPUTicks   uint64
	ProcessRSSBytes   uint64
	ProcessVSizeBytes uint64
	ProcessThreads    int
	ProcessUptimeSec  float64
	HostCPUIdleTicks  uint64
	HostCPUTotalTicks uint64
	SystemUptimeSec   float64
	Load1             float64
	Load5             float64
	Load15            float64
	MemTotalBytes     uint64
	MemAvailBytes     uint64
	DiskTotalBytes    uint64
	DiskFreeBytes     uint64
}

func (s *Server) handleWebHostMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sample := collectHostMonitorSample(s.svc.Cfg.DataDir, s.startedAt)
	cpuPct := s.computeProcessCPUPercent(sample.ProcessCPUTicks)
	hostCPUPct := s.computeHostCPUPercent(sample.HostCPUTotalTicks, sample.HostCPUIdleTicks)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	memUsed := uint64(0)
	memUsedPct := 0.0
	if sample.MemTotalBytes > 0 && sample.MemAvailBytes <= sample.MemTotalBytes {
		memUsed = sample.MemTotalBytes - sample.MemAvailBytes
		memUsedPct = percent(memUsed, sample.MemTotalBytes)
	}
	diskUsed := uint64(0)
	diskUsedPct := 0.0
	if sample.DiskTotalBytes > 0 && sample.DiskFreeBytes <= sample.DiskTotalBytes {
		diskUsed = sample.DiskTotalBytes - sample.DiskFreeBytes
		diskUsedPct = percent(diskUsed, sample.DiskTotalBytes)
	}

	hostname := sample.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	if sample.ProcessUptimeSec <= 0 {
		sample.ProcessUptimeSec = time.Since(s.startedAt).Seconds()
	}
	hostMetricsAvailable := sample.SystemUptimeSec > 0 || sample.MemTotalBytes > 0 || sample.DiskTotalBytes > 0 || sample.Load1 > 0 || sample.Load5 > 0 || sample.Load15 > 0 || sample.HostCPUTotalTicks > 0
	processMetricsAvailable := sample.ProcessCPUTicks > 0 || sample.ProcessRSSBytes > 0 || sample.ProcessVSizeBytes > 0 || sample.ProcessThreads > 0

	respondJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"host": map[string]any{
			"hostname":           hostname,
			"goos":               runtime.GOOS,
			"goarch":             runtime.GOARCH,
			"num_cpu":            runtime.NumCPU(),
			"cpu_percent":        hostCPUPct,
			"load_1":             sample.Load1,
			"load_5":             sample.Load5,
			"load_15":            sample.Load15,
			"load_supported":     runtime.GOOS != "windows",
			"uptime_seconds":     sample.SystemUptimeSec,
			"memory_total_bytes": sample.MemTotalBytes,
			"memory_avail_bytes": sample.MemAvailBytes,
			"memory_used_bytes":  memUsed,
			"memory_used_pct":    memUsedPct,
			"disk_total_bytes":   sample.DiskTotalBytes,
			"disk_free_bytes":    sample.DiskFreeBytes,
			"disk_used_bytes":    diskUsed,
			"disk_used_pct":      diskUsedPct,
			"metrics_available":  hostMetricsAvailable,
		},
		"process": map[string]any{
			"pid":               os.Getpid(),
			"cpu_percent":       cpuPct,
			"rss_bytes":         sample.ProcessRSSBytes,
			"vsize_bytes":       sample.ProcessVSizeBytes,
			"threads":           sample.ProcessThreads,
			"goroutines":        runtime.NumGoroutine(),
			"uptime_seconds":    sample.ProcessUptimeSec,
			"metrics_available": processMetricsAvailable,
		},
		"go_runtime": map[string]any{
			"version":              runtime.Version(),
			"heap_alloc_bytes":     mem.HeapAlloc,
			"heap_sys_bytes":       mem.HeapSys,
			"gc_cycles":            mem.NumGC,
			"total_alloc_bytes":    mem.TotalAlloc,
			"last_gc_unix_nano":    int64(mem.LastGC),
			"next_gc_target_bytes": mem.NextGC,
		},
	})
}

func (s *Server) computeProcessCPUPercent(procTicks uint64) float64 {
	if procTicks == 0 {
		return 0
	}
	now := time.Now()
	s.monitorMu.Lock()
	defer s.monitorMu.Unlock()
	prevTicks := s.monitorPrevProcTicks
	prevAt := s.monitorPrevAt
	s.monitorPrevProcTicks = procTicks
	s.monitorPrevAt = now
	if prevTicks == 0 || prevAt.IsZero() || procTicks < prevTicks {
		return 0
	}
	deltaTicks := procTicks - prevTicks
	deltaSec := now.Sub(prevAt).Seconds()
	if deltaSec <= 0 {
		return 0
	}
	cpuSec := float64(deltaTicks) / float64(hostClockTicks())
	if cpuSec < 0 {
		return 0
	}
	return clampFloat((cpuSec/deltaSec)*100, 0, float64(runtime.NumCPU()*100))
}

func (s *Server) computeHostCPUPercent(totalTicks, idleTicks uint64) float64 {
	if totalTicks == 0 {
		return 0
	}
	s.monitorMu.Lock()
	defer s.monitorMu.Unlock()
	prevTotal := s.monitorPrevHostTotal
	prevIdle := s.monitorPrevHostIdle
	s.monitorPrevHostTotal = totalTicks
	s.monitorPrevHostIdle = idleTicks
	if prevTotal == 0 || totalTicks < prevTotal || idleTicks < prevIdle {
		return 0
	}
	deltaTotal := totalTicks - prevTotal
	if deltaTotal == 0 {
		return 0
	}
	deltaIdle := idleTicks - prevIdle
	if deltaIdle > deltaTotal {
		deltaIdle = deltaTotal
	}
	busy := deltaTotal - deltaIdle
	return clampFloat((float64(busy)/float64(deltaTotal))*100, 0, 100)
}

func percent(v, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(v) / float64(total)) * 100
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func hostClockTicks() uint64 {
	// Linux default HZ is commonly 100 in container/VPS environments.
	return 100
}

//go:build linux

package httpapi

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func collectHostMonitorSample(dataDir string, startedAt time.Time) hostMonitorSample {
	out := hostMonitorSample{
		ProcessUptimeSec: time.Since(startedAt).Seconds(),
	}
	if h, err := os.Hostname(); err == nil {
		out.Hostname = h
	}
	if ticks, rss, vsize, threads, upSec, ok := readProcSelfStat(); ok {
		out.ProcessCPUTicks = ticks
		out.ProcessRSSBytes = rss
		out.ProcessVSizeBytes = vsize
		out.ProcessThreads = threads
		if upSec > 0 {
			out.ProcessUptimeSec = upSec
		}
	}
	if up, ok := readHostUptime(); ok {
		out.SystemUptimeSec = up
	}
	if totalTicks, idleTicks, ok := readHostCPUTimes(); ok {
		out.HostCPUTotalTicks = totalTicks
		out.HostCPUIdleTicks = idleTicks
	}
	if l1, l5, l15, ok := readLoadAvg(); ok {
		out.Load1, out.Load5, out.Load15 = l1, l5, l15
	}
	if total, avail, ok := readMemInfo(); ok {
		out.MemTotalBytes = total
		out.MemAvailBytes = avail
	}
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "."
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	if total, free, ok := readDiskUsage(dataDir); ok {
		out.DiskTotalBytes = total
		out.DiskFreeBytes = free
	}
	return out
}

func readProcSelfStat() (ticks uint64, rss uint64, vsize uint64, threads int, uptimeSec float64, ok bool) {
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return
	}
	txt := string(raw)
	end := strings.LastIndex(txt, ")")
	if end < 0 || end+2 >= len(txt) {
		return
	}
	rest := strings.Fields(txt[end+2:])
	if len(rest) < 22 {
		return
	}
	utime, err1 := strconv.ParseUint(rest[11], 10, 64)
	stime, err2 := strconv.ParseUint(rest[12], 10, 64)
	numThreads, err3 := strconv.Atoi(rest[17])
	startTicks, err4 := strconv.ParseUint(rest[19], 10, 64)
	vsizeBytes, err5 := strconv.ParseUint(rest[20], 10, 64)
	rssPages, err6 := strconv.ParseInt(rest[21], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil {
		return
	}
	hostUp, hasHostUp := readHostUptime()
	tps := float64(hostClockTicks())
	if hasHostUp && tps > 0 {
		startSec := float64(startTicks) / tps
		if hostUp > startSec {
			uptimeSec = hostUp - startSec
		}
	}
	pageSize := int64(os.Getpagesize())
	ticks = utime + stime
	vsize = vsizeBytes
	if rssPages > 0 {
		rss = uint64(rssPages * pageSize)
	}
	threads = numThreads
	ok = true
	return
}

func readHostUptime() (float64, bool) {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readLoadAvg() (float64, float64, float64, bool) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, false
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	l1, err1 := strconv.ParseFloat(fields[0], 64)
	l5, err2 := strconv.ParseFloat(fields[1], 64)
	l15, err3 := strconv.ParseFloat(fields[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return l1, l5, l15, true
}

func readHostCPUTimes() (total uint64, idle uint64, ok bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 {
		return 0, 0, false
	}
	line := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(line, "cpu ") {
		return 0, 0, false
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0, 0, false
	}
	var vals [10]uint64
	for i := 1; i < len(fields) && i <= len(vals); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals[i-1] = v
		total += v
	}
	// idle + iowait
	idle = vals[3] + vals[4]
	if total == 0 {
		return 0, 0, false
	}
	return total, idle, true
}

func readMemInfo() (totalBytes uint64, availBytes uint64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "MemTotal:") {
			totalBytes = parseMemInfoKiB(line) * 1024
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			availBytes = parseMemInfoKiB(line) * 1024
		}
	}
	if totalBytes == 0 {
		return
	}
	ok = true
	return
}

func parseMemInfoKiB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func readDiskUsage(path string) (total uint64, free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return
	}
	bsize := uint64(st.Bsize)
	total = st.Blocks * bsize
	free = st.Bavail * bsize
	ok = true
	return
}

//go:build windows

package httpapi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modPsapi                 = windows.NewLazySystemDLL("psapi.dll")
	modKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessMemoryInfo = modPsapi.NewProc("GetProcessMemoryInfo")
	procGetTickCount64       = modKernel32.NewProc("GetTickCount64")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = modKernel32.NewProc("GetDiskFreeSpaceExW")
	procGetProcessTimes      = modKernel32.NewProc("GetProcessTimes")
	procGetSystemTimes       = modKernel32.NewProc("GetSystemTimes")
	procCreateToolhelp32Snap = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procThread32First        = modKernel32.NewProc("Thread32First")
	procThread32Next         = modKernel32.NewProc("Thread32Next")
)

const (
	th32csSnapThread = 0x00000004
)

type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type threadEntry32 struct {
	Size           uint32
	CntUsage       uint32
	ThreadID       uint32
	OwnerProcessID uint32
	BasePri        int32
	DeltaPri       int32
	Flags          uint32
}

func collectHostMonitorSample(dataDir string, startedAt time.Time) hostMonitorSample {
	out := hostMonitorSample{
		ProcessUptimeSec: time.Since(startedAt).Seconds(),
		ProcessThreads:   maxIntLocal(1, runtime.GOMAXPROCS(0)),
	}
	if h, err := os.Hostname(); err == nil {
		out.Hostname = h
	}
	if up := float64(getTickCount64()) / 1000.0; up > 0 {
		out.SystemUptimeSec = up
	}
	if totalTicks, idleTicks, ok := readWindowsHostCPUTimes(); ok {
		out.HostCPUTotalTicks = totalTicks
		out.HostCPUIdleTicks = idleTicks
	}

	total, avail := readWindowsMemory()
	out.MemTotalBytes = total
	out.MemAvailBytes = avail

	if strings.TrimSpace(dataDir) == "" {
		dataDir = "."
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	if dTotal, dFree, ok := readWindowsDisk(dataDir); ok {
		out.DiskTotalBytes = dTotal
		out.DiskFreeBytes = dFree
	}

	if ticks, rss, vsize, ok := readWindowsProcessStats(); ok {
		out.ProcessCPUTicks = ticks
		out.ProcessRSSBytes = rss
		out.ProcessVSizeBytes = vsize
	}
	if threads, ok := readWindowsProcessThreadCount(uint32(os.Getpid())); ok {
		out.ProcessThreads = threads
	}
	return out
}

func getTickCount64() uint64 {
	r1, _, _ := procGetTickCount64.Call()
	return uint64(r1)
}

func readWindowsMemory() (total uint64, avail uint64) {
	var ex memoryStatusEx
	ex.Length = uint32(unsafe.Sizeof(ex))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ex)))
	if r1 == 0 {
		return 0, 0
	}
	return ex.TotalPhys, ex.AvailPhys
}

func readWindowsDisk(path string) (total uint64, free uint64, ok bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, false
	}
	var freeAvail uint64
	var totalBytes uint64
	var totalFree uint64
	r1, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r1 == 0 {
		return 0, 0, false
	}
	return totalBytes, totalFree, true
}

func readWindowsProcessStats() (ticks uint64, rss uint64, vsize uint64, ok bool) {
	h := windows.CurrentProcess()
	var create, exit, kernel, user windows.Filetime
	r1, _, _ := procGetProcessTimes.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&create)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r1 != 0 {
		total100ns := filetimeToUint64(kernel) + filetimeToUint64(user)
		// 100ns -> 10ms units, matching hostClockTicks()=100.
		ticks = total100ns / 100000
	}

	var pmc processMemoryCountersEx
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r1, _, _ = procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(pmc.CB),
	)
	if r1 == 0 {
		if ticks == 0 {
			return 0, 0, 0, false
		}
		return ticks, 0, 0, true
	}
	rss = uint64(pmc.WorkingSetSize)
	vsize = uint64(pmc.PrivateUsage)
	return ticks, rss, vsize, true
}

func readWindowsHostCPUTimes() (total uint64, idle uint64, ok bool) {
	var idleFT, kernelFT, userFT windows.Filetime
	r1, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if r1 == 0 {
		return 0, 0, false
	}
	idle = filetimeToUint64(idleFT)
	kernel := filetimeToUint64(kernelFT)
	user := filetimeToUint64(userFT)
	total = kernel + user
	if total == 0 {
		return 0, 0, false
	}
	return total, idle, true
}

func filetimeToUint64(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func maxIntLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func readWindowsProcessThreadCount(pid uint32) (int, bool) {
	snapshot, _, _ := procCreateToolhelp32Snap.Call(th32csSnapThread, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return 0, false
	}
	handle := windows.Handle(snapshot)
	defer windows.CloseHandle(handle)

	var entry threadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	r1, _, _ := procThread32First.Call(uintptr(handle), uintptr(unsafe.Pointer(&entry)))
	if r1 == 0 {
		return 0, false
	}
	count := 0
	for {
		if entry.OwnerProcessID == pid {
			count++
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		r1, _, _ = procThread32Next.Call(uintptr(handle), uintptr(unsafe.Pointer(&entry)))
		if r1 == 0 {
			break
		}
	}
	if count <= 0 {
		return 0, false
	}
	return count, true
}

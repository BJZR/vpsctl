package handlers

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// SystemHandler serves system information endpoints.
type SystemHandler struct {
	history *MetricsHistory
}

// NewSystemHandler creates a system handler with a metrics history buffer.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{history: NewMetricsHistory(60)}
}

// infoResponse is the /api/system/info payload.
type infoResponse struct {
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Kernel      string    `json:"kernel"`
	Uptime      float64   `json:"uptime"`
	LoadAverage []float64 `json:"load_average"`
	CPUModel    string    `json:"cpu_model,omitempty"`
	MemTotal    uint64    `json:"mem_total"`
}

// Info handles GET /api/system/info.
func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	var uinfo syscall.Utsname
	var osName, kernel string
	if err := syscall.Uname(&uinfo); err == nil {
		osName = int8ToString(uinfo.Sysname[:])
		kernel = int8ToString(uinfo.Release[:])
	}

	var si syscall.Sysinfo_t
	syscall.Sysinfo(&si)

	load := []float64{
		float64(si.Loads[0]) / 65536.0,
		float64(si.Loads[1]) / 65536.0,
		float64(si.Loads[2]) / 65536.0,
	}

	cpuModel := readCPUModel()

	writeJSON(w, http.StatusOK, infoResponse{
		Hostname:    hostname,
		OS:          osName,
		Kernel:      kernel,
		Uptime:      uptime(),
		LoadAverage: load,
		CPUModel:    cpuModel,
		MemTotal:    uint64(si.Totalram) * uint64(si.Unit),
	})
}

func readCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

// diskStat holds usage for a single mount.
type diskStat struct {
	Mountpoint string  `json:"mountpoint"`
	Filesystem string  `json:"filesystem"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	UsedPct    float64 `json:"used_percent"`
}

// metricsResponse is the /api/system/metrics payload.
type metricsResponse struct {
	CPU    cpuStats    `json:"cpu"`
	Memory memoryStats `json:"memory"`
	Swap   memoryStats `json:"swap"`
	Disks  []diskStat  `json:"disks"`
	Time   time.Time   `json:"time"`
}

type cpuStats struct {
	Used   float64 `json:"used_percent"`
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
}

type memoryStats struct {
	Total uint64  `json:"total"`
	Used  uint64  `json:"used"`
	Free  uint64  `json:"free"`
	Pct   float64 `json:"used_percent"`
}

// Metrics handles GET /api/system/metrics.
func (h *SystemHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	cpu := readCPU()
	mem, swap := readMemory()
	disks := readDisks()

	resp := metricsResponse{
		CPU:    cpu,
		Memory: mem,
		Swap:   swap,
		Disks:  disks,
		Time:   time.Now(),
	}
	h.history.Add(resp)

	writeJSON(w, http.StatusOK, resp)
}

// MetricsHistory handles GET /api/system/metrics/history.
func (h *SystemHandler) MetricsHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": h.history.All(),
	})
}

// networkInterface describes a single network interface.
type networkInterface struct {
	Name    string   `json:"name"`
	MAC     string   `json:"mac"`
	IPs     []string `json:"ips"`
	RXBytes uint64   `json:"rx_bytes"`
	TXBytes uint64   `json:"tx_bytes"`
}

// Network handles GET /api/system/network.
func (h *SystemHandler) Network(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	devStats := readNetDev()
	result := make([]networkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs := []string{}
		addrList, err := iface.Addrs()
		if err == nil {
			for _, a := range addrList {
				addrs = append(addrs, a.String())
			}
		}
		st := devStats[iface.Name]
		result = append(result, networkInterface{
			Name:    iface.Name,
			MAC:     iface.HardwareAddr.String(),
			IPs:     addrs,
			RXBytes: st.rx,
			TXBytes: st.tx,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

// processEntry describes one process in the process list.
type processEntry struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu_percent"`
	Mem  float64 `json:"mem_percent"`
	User string  `json:"user"`
}

// Processes handles GET /api/system/processes. Returns top 20 by CPU.
func (h *SystemHandler) Processes(w http.ResponseWriter, r *http.Request) {
	entries := listProcesses()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CPU > entries[j].CPU
	})
	if len(entries) > 20 {
		entries = entries[:20]
	}
	writeJSON(w, http.StatusOK, entries)
}

// --- MetricsHistory ring buffer ---

// MetricsHistory stores the last N metric snapshots.
type MetricsHistory struct {
	mu     sync.Mutex
	buffer []metricsResponse
	max    int
}

// NewMetricsHistory creates a ring buffer holding `max` snapshots.
func NewMetricsHistory(max int) *MetricsHistory {
	return &MetricsHistory{buffer: make([]metricsResponse, 0, max), max: max}
}

// Add appends a snapshot, trimming to the ring buffer size.
func (mh *MetricsHistory) Add(m metricsResponse) {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	mh.buffer = append(mh.buffer, m)
	if len(mh.buffer) > mh.max {
		over := len(mh.buffer) - mh.max
		mh.buffer = append([]metricsResponse{}, mh.buffer[over:]...)
	}
}

// All returns a shallow copy of the buffer.
func (mh *MetricsHistory) All() []metricsResponse {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	out := make([]metricsResponse, len(mh.buffer))
	copy(out, mh.buffer)
	return out
}

// --- low-level collectors ---

var (
	lastCPUOnce sync.Once
	lastCPUTime cpuTimes
)

type cpuTimes struct {
	user   uint64
	nice   uint64
	system uint64
	idle   uint64
	total  uint64
}

func readCPU() cpuStats {
	t := readCPUTimes()
	delta := t.total - lastCPUTime.total
	cur := cpuStats{}
	if delta > 0 {
		cur.User = 100 * float64(t.user-lastCPUTime.user) / float64(delta)
		cur.System = 100 * float64(t.system-lastCPUTime.system) / float64(delta)
		cur.Idle = 100 * float64(t.idle-lastCPUTime.idle) / float64(delta)
		cur.Used = cur.User + cur.System
	}
	lastCPUTime = t
	return cur
}

func readCPUTimes() cpuTimes {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		vals := make([]uint64, 0, len(fields)-1)
		for _, f := range fields[1:] {
			n, _ := strconv.ParseUint(f, 10, 64)
			vals = append(vals, n)
		}
		if len(vals) < 4 {
			return cpuTimes{}
		}
		var total uint64
		for _, v := range vals {
			total += v
		}
		return cpuTimes{
			user:   vals[0],
			nice:   vals[1],
			system: vals[2],
			idle:   vals[3],
			total:  total,
		}
	}
	return cpuTimes{}
}

func readMemory() (mem, swap memoryStats) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	var kbTotal, kbFree, kbAvail, kbBuffers, kbCached, kbSwapTotal, kbSwapFree uint64
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			kbTotal = v
		case "MemFree":
			kbFree = v
		case "MemAvailable":
			kbAvail = v
		case "Buffers":
			kbBuffers = v
		case "Cached":
			kbCached = v
		case "SwapTotal":
			kbSwapTotal = v
		case "SwapFree":
			kbSwapFree = v
		}
	}

	if kbAvail == 0 {
		kbAvail = kbFree + kbBuffers + kbCached
	}
	used := kbTotal - kbAvail
	mem = memoryStats{
		Total: kbTotal * 1024,
		Used:  used * 1024,
		Free:  kbAvail * 1024,
		Pct:   pct(used, kbTotal),
	}
	swapUsed := kbSwapTotal - kbSwapFree
	swap = memoryStats{
		Total: kbSwapTotal * 1024,
		Used:  swapUsed * 1024,
		Free:  kbSwapFree * 1024,
		Pct:   pct(swapUsed, kbSwapTotal),
	}
	return
}

func pct(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}

func readDisks() []diskStat {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	var mounts []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		fs, mount := fields[0], fields[1]
		// Only include real block/overlay mounts, skip pseudo filesystems.
		if strings.HasPrefix(mount, "/dev") || strings.HasPrefix(fs, "/dev/") ||
			strings.HasPrefix(mount, "/") && (strings.Contains(fields[2], "ext") || strings.Contains(fields[2], "xfs") || strings.Contains(fields[2], "btrfs") || strings.Contains(fields[2], "overlay") || strings.Contains(fields[2], "f2fs")) {
			mounts = append(mounts, mount)
		}
	}

	seen := map[string]bool{}
	var out []diskStat
	for _, m := range mounts {
		if seen[m] {
			continue
		}
		seen[m] = true
		var st syscall.Statfs_t
		if err := syscall.Statfs(m, &st); err != nil {
			continue
		}
		bsize := uint64(st.Bsize)
		total := st.Blocks * bsize
		free := st.Bavail * bsize
		used := total - free
		// Also compute used as blocks-free * bsize classification.
		usedAll := (st.Blocks - st.Bfree) * bsize
		if usedAll > used {
			used = usedAll
		}
		out = append(out, diskStat{
			Mountpoint: m,
			Filesystem: fsFor(m),
			Total:      total,
			Used:       used,
			Free:       free,
			UsedPct:    pct(used, total),
		})
	}
	return out
}

func fsFor(mount string) string {
	return mount
}

type netStat struct{ rx, tx uint64 }

func readNetDev() map[string]netStat {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}
	out := map[string]netStat{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		out[name] = netStat{rx: rx, tx: tx}
	}
	return out
}

func listProcesses() []processEntry {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	memTotal := totalMemBytes()

	var result []processEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}
		// Parse name out of stat (may contain spaces/parens).
		statStr := string(stat)
		open := strings.Index(statStr, "(")
		close := strings.Index(statStr, ")")
		if open < 0 || close < 0 {
			continue
		}
		name := statStr[open+1 : close]
		rest := strings.Fields(statStr[close+1:])

		// rest[1] = state, rest[13] = utime, rest[14] = stime, rest[21] = starttime
		var utime, stime, starttime uint64
		if len(rest) > 22 {
			utime, _ = strconv.ParseUint(rest[13], 10, 64)
			stime, _ = strconv.ParseUint(rest[14], 10, 64)
			starttime, _ = strconv.ParseUint(rest[21], 10, 64)
		}

		rss := readStatm(pid)
		user := readProcessUser(pid)

		pctCPU := 0.0
		if uptimeSecs := uptime(); uptimeSecs > 0 {
			// Approximate CPU% = (utime+stime)/HZ / processAge * 100
			hz := float64(100)
			processAge := uptimeSecs - float64(starttime)/hz
			if processAge > 0 {
				pctCPU = float64(utime+stime) / hz / processAge * 100
			}
		}

		memPct := 0.0
		if memTotal > 0 {
			memPct = float64(rss) * 100 / float64(memTotal)
		}

		result = append(result, processEntry{
			PID:  pid,
			Name: name,
			User: user,
			CPU:  pctCPU,
			Mem:  memPct,
		})
	}
	return result
}

func readStatm(pid int) (rssPages uint64) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 2 {
		rssPages, _ = strconv.ParseUint(fields[1], 10, 64)
	}
	return
}

func readProcessUser(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ := strconv.Atoi(fields[1])
				return userForUID(uid)
			}
			return ""
		}
	}
	return ""
}

var uidCache sync.Map

func userForUID(uid int) string {
	if v, ok := uidCache.Load(uid); ok {
		return v.(string)
	}
	out, _, err := runCommand("id", "-un", strconv.Itoa(uid))
	user := strings.TrimSpace(out)
	if err != nil || user == "" {
		user = strconv.Itoa(uid)
	}
	uidCache.Store(uid, user)
	return user
}

func totalMemBytes() uint64 {
	mem, _ := readMemory()
	return mem.Total
}

func uptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func int8ToString(arr []int8) string {
	b := make([]byte, 0, len(arr))
	for _, c := range arr {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

package collector

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"

	"zfstool/internal/execzfs"
	"zfstool/internal/version"
)

// Host collects OS + ZFS version + ARC basics.
type Host struct {
	Hostname       string
	OSName         string
	OSVersion      string
	Kernel         string
	Arch           string
	UptimeSeconds  float64
	MemTotalKB     uint64
	MemAvailableKB uint64
	ZFSVersions    []string
	ZFSMismatch    bool
	ARC            map[string]uint64
}

func CollectHost(ctx context.Context) (*Host, error) {
	h := &Host{ARC: make(map[string]uint64)}
	if hn, err := os.Hostname(); err == nil {
		h.Hostname = hn
	}
	h.Arch = runtime.GOARCH
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		h.Kernel = strings.TrimSpace(string(b))
	}
	h.readOSRelease()
	h.readUptime()
	h.readMeminfo()
	_ = h.readZFSVersion(ctx)
	h.readARCStats()
	return h, nil
}

func (h *Host) readOSRelease() {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "NAME=") {
			h.OSName = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
		}
		if strings.HasPrefix(line, "VERSION=") {
			h.OSVersion = strings.Trim(strings.TrimPrefix(line, "VERSION="), `"`)
		}
	}
}

func (h *Host) readUptime() {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return
	}
	sec, err := strconv.ParseFloat(fields[0], 64)
	if err == nil {
		h.UptimeSeconds = sec
	}
}

func (h *Host) readMeminfo() {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				h.MemTotalKB = v
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				h.MemAvailableKB = v
			}
		}
	}
}

func (h *Host) readZFSVersion(ctx context.Context) error {
	out, err := execzfs.RunZfs(ctx, "version")
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var user, kernel string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "zfs-") {
			user = ln
		}
		if strings.Contains(ln, "zfs-kmod") || strings.Contains(ln, "ZFS") && strings.Contains(ln, "module") {
			kernel = ln
		}
	}
	if user != "" {
		h.ZFSVersions = append(h.ZFSVersions, user)
	}
	if kernel != "" {
		h.ZFSVersions = append(h.ZFSVersions, kernel)
	}
	if user != "" && kernel != "" {
		u := extractVer(user)
		k := extractVer(kernel)
		if u != "" && k != "" && u != k {
			h.ZFSMismatch = true
		}
	}
	return nil
}

func extractVer(s string) string {
	// crude: first token looking like x.y.z
	for _, tok := range strings.Fields(s) {
		if strings.Count(tok, ".") >= 1 && len(tok) >= 3 {
			return tok
		}
	}
	return ""
}

func (h *Host) readARCStats() {
	b, err := os.ReadFile("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		fields := strings.Fields(ln)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		val, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "size", "hits", "misses", "c", "p", "mfu_size", "mru_size":
			h.ARC[name] = val
		}
	}
}

// AgentVersion returns embedded version.
func AgentVersion() string { return version.Version }

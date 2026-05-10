package collector

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/nixomose/zfstool/internal/api"
)

// SMARTJSON runs smartctl -a -j device (best effort).
func SMARTJSON(ctx context.Context, device string) (*api.SMARTDisk, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "smartctl", "-a", "-j", device)
	out, err := cmd.Output()
	d := &api.SMARTDisk{Device: device}
	if err != nil {
		d.Error = err.Error()
		return d, nil
	}
	var m map[string]interface{}
	if json.Unmarshal(out, &m) == nil {
		d.JSON = m
	}
	return d, nil
}

// blockDeviceShortName matches Linux whole-disk or partition leaf names (no /dev/ prefix).
var blockDeviceShortName = regexp.MustCompile(
	`^(sd[a-z]+[0-9]*|vd[a-z]+[0-9]*|xvd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|md[0-9]+|dm-[0-9]+|mmcblk[0-9]+(p[0-9]+)?|loop[0-9]+)$`)

// MapVDevToDevices extracts block device paths from the status config tree: leaf vdevs
// that are disks (not mirror/raidz/draid containers). Names may be /dev/... (zpool status -P)
// or short forms like sda1; paths are normalized for smartctl.
func MapVDevToDevices(config []api.StatusLine) []string {
	var devs []string
	seen := map[string]struct{}{}
	for i := range config {
		if !isStatusConfigLeaf(config, i) {
			continue
		}
		n := config[i].Name
		if !isDiskLeafName(n) {
			continue
		}
		path := normalizeDiskPath(n)
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			devs = append(devs, path)
		}
	}
	return devs
}

func isStatusConfigLeaf(config []api.StatusLine, idx int) bool {
	if idx < 0 || idx >= len(config) {
		return false
	}
	ind := config[idx].Indent
	if idx+1 >= len(config) {
		return true
	}
	return config[idx+1].Indent <= ind
}

func isVdevAggregateName(n string) bool {
	lower := strings.ToLower(strings.TrimSpace(n))
	switch lower {
	case "logs", "cache", "spares", "special", "dedup", "$free":
		return true
	}
	if strings.HasPrefix(lower, "mirror") {
		return true
	}
	if strings.Contains(lower, "raidz") {
		return true
	}
	if strings.Contains(lower, "draid") {
		return true
	}
	if strings.HasPrefix(lower, "replacing-") {
		return true
	}
	return false
}

func isByIdStyleDiskName(n string) bool {
	lower := strings.ToLower(n)
	return strings.Contains(lower, "ata-") ||
		strings.Contains(lower, "nvme-") ||
		strings.Contains(lower, "wwn-") ||
		strings.Contains(lower, "scsi-") ||
		strings.Contains(lower, "usb-")
}

func isDiskLeafName(n string) bool {
	if n == "" {
		return false
	}
	if isVdevAggregateName(n) {
		return false
	}
	if strings.HasPrefix(n, "/dev/") {
		return true
	}
	if isByIdStyleDiskName(n) {
		return true
	}
	return blockDeviceShortName.MatchString(n)
}

func normalizeDiskPath(n string) string {
	if strings.HasPrefix(n, "/dev/") {
		return n
	}
	if strings.HasPrefix(n, "/") {
		return n
	}
	if strings.Contains(n, "/") {
		return "/dev/" + n
	}
	return "/dev/" + n
}

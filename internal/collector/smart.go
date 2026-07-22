package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nixomose/zfstool/internal/api"
)

// ErrInvalidDevice is returned when a SMART device path escapes /dev or is malformed.
var ErrInvalidDevice = errors.New("invalid block device path")

// ValidateBlockDevicePath normalizes and restricts a device path to under /dev/.
// Rejects path traversal (e.g. /dev/../../etc/passwd) and symlink escapes out of /dev.
func ValidateBlockDevicePath(device string) (string, error) {
	device = strings.TrimSpace(device)
	if device == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidDevice)
	}
	if strings.ContainsRune(device, 0) || strings.ContainsAny(device, "\n\r") {
		return "", fmt.Errorf("%w: control characters", ErrInvalidDevice)
	}
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + strings.TrimPrefix(device, "/")
	}
	cleaned := filepath.Clean(device)
	if !isUnderDev(cleaned) {
		return "", fmt.Errorf("%w: must be under /dev", ErrInvalidDevice)
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		resolved = filepath.Clean(resolved)
		if !isUnderDev(resolved) {
			return "", fmt.Errorf("%w: symlink escapes /dev", ErrInvalidDevice)
		}
		return resolved, nil
	}
	// Non-existent devices are OK (smartctl reports the failure); keep cleaned path.
	if _, statErr := os.Lstat(cleaned); statErr != nil && os.IsNotExist(statErr) {
		return cleaned, nil
	}
	// Broken symlink or unreadable: still allow if cleaned stays under /dev.
	return cleaned, nil
}

func isUnderDev(p string) bool {
	return strings.HasPrefix(p, "/dev/") && p != "/dev/"
}

// SMARTJSON runs smartctl -a -j device (best effort).
// device must pass ValidateBlockDevicePath.
func SMARTJSON(ctx context.Context, device string) (*api.SMARTDisk, error) {
	safe, err := ValidateBlockDevicePath(device)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// "--" is ignored by smartctl but documents intent; device is already validated.
	cmd := exec.CommandContext(ctx, "smartctl", "-a", "-j", safe)
	out, err := cmd.Output()
	d := &api.SMARTDisk{Device: safe}
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
		// Non-/dev absolute paths are not used for smartctl from the API;
		// keep as-is only for display of unusual vdev names.
		return n
	}
	return "/dev/" + n
}

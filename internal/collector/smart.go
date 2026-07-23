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
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nixomose/zfstool/internal/api"
)

// ErrInvalidDevice is returned when a SMART device path escapes /dev or is malformed.
var ErrInvalidDevice = errors.New("invalid block device path")

// SmartctlCmd is the smartctl binary name/path (overridable in tests).
var SmartctlCmd = "smartctl"

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

// SMARTJSON runs smartctl -a -j on device (best effort).
// device must pass ValidateBlockDevicePath. Partition paths are remapped to the
// whole disk because SMART is a drive-level property.
//
// smartctl uses a bitmask exit status: non-zero often means "found problems" or
// a soft command failure while still printing valid JSON. We always parse stdout.
func SMARTJSON(ctx context.Context, device string) (*api.SMARTDisk, error) {
	safe, err := ValidateBlockDevicePath(device)
	if err != nil {
		return nil, err
	}
	query := wholeDiskForSMART(safe)
	if query != safe {
		if q, qerr := ValidateBlockDevicePath(query); qerr == nil {
			query = q
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmdName := SmartctlCmd
	if _, lookErr := exec.LookPath(cmdName); lookErr != nil && cmdName == "smartctl" {
		// Desktop sessions sometimes omit /usr/sbin from PATH.
		if _, e := os.Stat("/usr/sbin/smartctl"); e == nil {
			cmdName = "/usr/sbin/smartctl"
		}
	}
	cmd := exec.CommandContext(ctx, cmdName, "-a", "-j", query)
	out, err := cmd.Output()
	d := &api.SMARTDisk{Device: safe}

	var m map[string]interface{}
	if len(out) > 0 && json.Unmarshal(out, &m) == nil {
		if hasSMARTPayload(m) {
			d.JSON = m
			// Non-zero exit with real SMART data is normal (health bitmask bits).
			return d, nil
		}
	}
	if err == nil {
		if m != nil {
			d.JSON = m
		}
		return d, nil
	}
	d.Error, d.ErrorKind = classifySmartctlError(err, m)
	return d, nil
}

// wholeDiskForSMART maps a partition node to its parent disk for smartctl.
func wholeDiskForSMART(dev string) string {
	dir := filepath.Dir(dev)
	base := filepath.Base(dev)

	// /dev/disk/by-id/...-partN (and by-path equivalents)
	if i := strings.LastIndex(base, "-part"); i > 0 {
		suf := base[i+len("-part"):]
		if suf != "" && isAllDigits(suf) {
			return filepath.Join(dir, base[:i])
		}
	}

	// nvme0n1p2 → nvme0n1 (do not strip the namespace digit from nvme0n1).
	if strings.HasPrefix(base, "nvme") {
		if m := nvmePartRE.FindStringSubmatch(base); m != nil {
			return filepath.Join(dir, m[1])
		}
		return dev
	}
	// mmcblk0p1 → mmcblk0
	if strings.HasPrefix(base, "mmcblk") {
		if m := mmcPartRE.FindStringSubmatch(base); m != nil {
			return filepath.Join(dir, m[1])
		}
		return dev
	}

	// sdX1 / vdX1 / xvdX1 — trailing digits are the partition; mdN/loopN/dm-N keep digits.
	if strings.HasPrefix(base, "md") || strings.HasPrefix(base, "dm-") || strings.HasPrefix(base, "loop") {
		return dev
	}
	i := len(base)
	for i > 0 && unicode.IsDigit(rune(base[i-1])) {
		i--
	}
	if i > 0 && i < len(base) {
		return filepath.Join(dir, base[:i])
	}
	return dev
}

var (
	nvmePartRE = regexp.MustCompile(`^(nvme\d+n\d+)p\d+$`)
	mmcPartRE  = regexp.MustCompile(`^(mmcblk\d+)p\d+$`)
)

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// hasSMARTPayload reports whether smartctl JSON includes drive identity or SMART fields
// (as opposed to an error-only envelope with json_format_version / smartctl / local_time).
func hasSMARTPayload(m map[string]interface{}) bool {
	if m == nil {
		return false
	}
	for _, k := range []string{
		"smart_status",
		"ata_smart_attributes",
		"ata_smart_data",
		"ata_smart_error_log",
		"ata_smart_self_test_log",
		"nvme_smart_health_information_log",
		"nvme_controller_identify",
		"scsi_error_counter_log",
		"scsi_grown_defect_list",
		"temperature",
		"power_on_time",
		"model_name",
		"scsi_model_name",
		"serial_number",
		"user_capacity",
		"firmware_version",
		"model_family",
	} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// classifySmartctlError returns a user-facing message and an api.SMARTError* kind.
func classifySmartctlError(err error, m map[string]interface{}) (string, string) {
	if errors.Is(err, exec.ErrNotFound) || (err != nil && strings.Contains(err.Error(), "executable file not found")) {
		return "smartctl not found; install the smartmontools package", api.SMARTErrorNotFound
	}

	msg := firstSmartctlMessage(m)
	code := smartctlExitCode(err, m)

	if code&2 != 0 || looksLikePermissionError(msg) {
		if msg == "" {
			if code != 0 {
				msg = fmt.Sprintf("cannot open device (smartctl exit status %d)", code)
			} else {
				msg = "cannot open device"
			}
		}
		msg = appendDiskGroupHint(msg)
		return msg, api.SMARTErrorPermission
	}

	if msg == "" {
		if code != 0 {
			msg = fmt.Sprintf("smartctl exit status %d", code)
		} else if err != nil {
			msg = err.Error()
		} else {
			msg = "smartctl failed"
		}
	}
	return msg, api.SMARTErrorFailed
}

func smartctlExitCode(err error, m map[string]interface{}) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	if m == nil {
		return 0
	}
	sc, _ := m["smartctl"].(map[string]interface{})
	if sc == nil {
		return 0
	}
	switch v := sc["exit_status"].(type) {
	case float64:
		return int(v)
	case json.Number:
		if n, e := strconv.Atoi(string(v)); e == nil {
			return n
		}
	}
	return 0
}

func looksLikePermissionError(msg string) bool {
	low := strings.ToLower(msg)
	return strings.Contains(low, "permission denied") ||
		strings.Contains(low, "operation not permitted")
}

func appendDiskGroupHint(msg string) string {
	low := strings.ToLower(msg)
	if strings.Contains(low, "disk group") {
		return msg
	}
	return msg + "; if not running as root, add your user to the disk group (sudo usermod -aG disk $USER) and log out/in"
}

func firstSmartctlMessage(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	sc, _ := m["smartctl"].(map[string]interface{})
	if sc == nil {
		return ""
	}
	msgs, _ := sc["messages"].([]interface{})
	for _, raw := range msgs {
		msg, _ := raw.(map[string]interface{})
		if msg == nil {
			continue
		}
		s, _ := msg["string"].(string)
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return ""
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

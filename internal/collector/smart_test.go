package collector

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nixomose/zfstool/internal/api"
)

func TestValidateBlockDevicePath(t *testing.T) {
	ok, err := ValidateBlockDevicePath("sda")
	if err != nil {
		t.Fatal(err)
	}
	if ok != "/dev/sda" {
		t.Fatalf("got %q", ok)
	}

	ok, err = ValidateBlockDevicePath("/dev/sdb1")
	if err != nil {
		t.Fatal(err)
	}
	if ok != "/dev/sdb1" {
		t.Fatalf("got %q", ok)
	}

	// Path traversal must not escape /dev.
	if _, err := ValidateBlockDevicePath("../../etc/passwd"); err == nil {
		t.Fatal("expected rejection of ../../etc/passwd")
	} else if !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("want ErrInvalidDevice, got %v", err)
	}
	if _, err := ValidateBlockDevicePath("/dev/../../etc/passwd"); err == nil {
		t.Fatal("expected rejection of /dev/../../etc/passwd")
	}

	if _, err := ValidateBlockDevicePath(""); err == nil {
		t.Fatal("expected empty rejection")
	}

	// by-id style under /dev (may not exist — still accepted if cleaned stays under /dev)
	p := "/dev/disk/by-id/ata-EXAMPLE"
	got, err := ValidateBlockDevicePath(p)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(p) && got != p {
		// If the path exists as a symlink, EvalSymlinks may rewrite; either way under /dev.
		if !isUnderDev(got) {
			t.Fatalf("escaped /dev: %q", got)
		}
	}
}

func TestValidateBlockDevicePathEncodedTraversal(t *testing.T) {
	// Same logical path the HTTP handler receives after PathValue unescapes %2F.
	if _, err := ValidateBlockDevicePath("../../etc/passwd"); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("got %v", err)
	}
}

func TestWholeDiskForSMART(t *testing.T) {
	cases := map[string]string{
		"/dev/sda":                              "/dev/sda",
		"/dev/sda1":                             "/dev/sda",
		"/dev/sdb12":                            "/dev/sdb",
		"/dev/vdb1":                             "/dev/vdb",
		"/dev/nvme0n1":                          "/dev/nvme0n1",
		"/dev/nvme0n1p3":                        "/dev/nvme0n1",
		"/dev/mmcblk0p1":                        "/dev/mmcblk0",
		"/dev/md0":                              "/dev/md0",
		"/dev/dm-0":                             "/dev/dm-0",
		"/dev/loop0":                            "/dev/loop0",
		"/dev/disk/by-id/ata-FOO":               "/dev/disk/by-id/ata-FOO",
		"/dev/disk/by-id/ata-FOO-part1":         "/dev/disk/by-id/ata-FOO",
		"/dev/disk/by-id/nvme-BAR-part12":       "/dev/disk/by-id/nvme-BAR",
	}
	for in, want := range cases {
		if got := wholeDiskForSMART(in); got != want {
			t.Errorf("wholeDiskForSMART(%q)=%q want %q", in, got, want)
		}
	}
}

func TestHasSMARTPayload(t *testing.T) {
	if hasSMARTPayload(nil) {
		t.Fatal("nil")
	}
	if hasSMARTPayload(map[string]interface{}{
		"json_format_version": []interface{}{1, 0},
		"smartctl":            map[string]interface{}{"exit_status": float64(4)},
	}) {
		t.Fatal("error-only envelope should not count as payload")
	}
	if !hasSMARTPayload(map[string]interface{}{
		"smart_status": map[string]interface{}{"passed": true},
	}) {
		t.Fatal("smart_status should count")
	}
}

func TestSMARTJSONKeepsDataOnExitStatus4(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "smartctl")
	// Classic smartctl: non-zero bitmask exit with valid SMART JSON on stdout.
	body := `#!/bin/sh
cat <<'EOF'
{
  "json_format_version": [1, 0],
  "smartctl": {"exit_status": 4},
  "model_name": "TEST DISK",
  "serial_number": "SN123",
  "smart_status": {"passed": true},
  "temperature": {"current": 32}
}
EOF
exit 4
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := SmartctlCmd
	SmartctlCmd = script
	t.Cleanup(func() { SmartctlCmd = old })

	d, err := SMARTJSON(context.Background(), "/dev/sda1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Error != "" {
		t.Fatalf("unexpected error %q (should keep JSON despite exit 4)", d.Error)
	}
	if d.JSON == nil {
		t.Fatal("expected JSON")
	}
	if d.JSON["model_name"] != "TEST DISK" {
		t.Fatalf("model_name=%v", d.JSON["model_name"])
	}
	// Queried whole disk but Device stays as validated request path.
	if d.Device != "/dev/sda1" {
		t.Fatalf("Device=%q", d.Device)
	}
}

func TestSMARTJSONPermissionMessage(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "smartctl")
	body := `#!/bin/sh
cat <<'EOF'
{
  "json_format_version": [1, 0],
  "smartctl": {
    "exit_status": 2,
    "messages": [{"string": "Smartctl open device: /dev/sda failed: Permission denied", "severity": "error"}]
  }
}
EOF
exit 2
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := SmartctlCmd
	SmartctlCmd = script
	t.Cleanup(func() { SmartctlCmd = old })

	d, err := SMARTJSON(context.Background(), "sda")
	if err != nil {
		t.Fatal(err)
	}
	if d.JSON != nil {
		t.Fatalf("expected no payload JSON, got %v", d.JSON)
	}
	if !strings.Contains(d.Error, "Permission denied") {
		t.Fatalf("error=%q", d.Error)
	}
	if !strings.Contains(d.Error, "disk group") {
		t.Fatalf("expected disk group hint in %q", d.Error)
	}
	if d.ErrorKind != api.SMARTErrorPermission {
		t.Fatalf("ErrorKind=%q", d.ErrorKind)
	}
}

func TestSMARTJSONOpenFailedWithoutMessage(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "smartctl")
	body := `#!/bin/sh
echo '{"json_format_version":[1,0],"smartctl":{"exit_status":2}}'
exit 2
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := SmartctlCmd
	SmartctlCmd = script
	t.Cleanup(func() { SmartctlCmd = old })

	d, err := SMARTJSON(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Error, "disk group") {
		t.Fatalf("error=%q", d.Error)
	}
	if d.ErrorKind != api.SMARTErrorPermission {
		t.Fatalf("ErrorKind=%q", d.ErrorKind)
	}
}

func TestClassifySmartctlErrorNotFound(t *testing.T) {
	msg, kind := classifySmartctlError(exec.ErrNotFound, nil)
	if !strings.Contains(msg, "smartmontools") {
		t.Fatalf("got %q", msg)
	}
	if kind != api.SMARTErrorNotFound {
		t.Fatalf("kind=%q", kind)
	}
}

func TestClassifySmartctlErrorFailed(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "smartctl")
	body := `#!/bin/sh
cat <<'EOF'
{
  "json_format_version": [1, 0],
  "smartctl": {
    "exit_status": 1,
    "messages": [{"string": "/dev/vdb: Unable to detect device type", "severity": "error"}]
  }
}
EOF
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := SmartctlCmd
	SmartctlCmd = script
	t.Cleanup(func() { SmartctlCmd = old })

	d, err := SMARTJSON(context.Background(), "/dev/vdb")
	if err != nil {
		t.Fatal(err)
	}
	if d.ErrorKind != api.SMARTErrorFailed {
		t.Fatalf("ErrorKind=%q error=%q", d.ErrorKind, d.Error)
	}
	if strings.Contains(d.Error, "smartmontools") {
		t.Fatalf("should not suggest package install: %q", d.Error)
	}
	if !strings.Contains(d.Error, "Unable to detect device type") {
		t.Fatalf("error=%q", d.Error)
	}
}

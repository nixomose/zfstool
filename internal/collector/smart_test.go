package collector

import (
	"errors"
	"path/filepath"
	"testing"
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

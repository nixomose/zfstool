package collector

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nixomose/zfstool/internal/api"
)

func TestConfinedJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "ok")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := confinedJoin(root, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if got != inside && got != filepath.Clean(inside) {
		// EvalSymlinks may normalize; ensure still under root.
		rel, e := filepath.Rel(root, got)
		if e != nil || rel == ".." || len(rel) >= 2 && rel[:2] == ".." {
			t.Fatalf("unexpected path %q", got)
		}
	}

	cases := []string{"..", "../", "foo/../../etc", "a/../../../etc/passwd", "/etc/passwd"}
	for _, rel := range cases {
		if _, err := confinedJoin(root, rel); !errors.Is(err, ErrInvalidBrowse) {
			t.Fatalf("rel %q: want ErrInvalidBrowse, got %v", rel, err)
		}
	}
}

func TestConfinedJoinRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := confinedJoin(root, "escape"); !errors.Is(err, ErrInvalidBrowse) {
		t.Fatalf("want ErrInvalidBrowse for symlink escape, got %v", err)
	}
}

func TestNormalizeMountTarget(t *testing.T) {
	ok, err := normalizeMountTarget("/boot")
	if err != nil || ok != "/boot" {
		t.Fatalf("got %q %v", ok, err)
	}
	root, err := normalizeMountTarget("/")
	if err != nil || root != "/" {
		t.Fatalf("root %q %v", root, err)
	}
	cleaned, err := normalizeMountTarget("/boot/foo/..")
	if err != nil || cleaned != "/boot" {
		t.Fatalf("clean %q %v", cleaned, err)
	}
	for _, bad := range []string{"", "boot", "relative", "/foo\nbar"} {
		if _, err := normalizeMountTarget(bad); !errors.Is(err, ErrInvalidBrowse) {
			t.Fatalf("%q: want ErrInvalidBrowse, got %v", bad, err)
		}
	}
}

func TestLookupMount(t *testing.T) {
	mounts := []api.MountEntry{
		{Target: "/", Fstype: "ext4"},
		{Target: "/boot", Fstype: "vfat"},
	}
	if lookupMount("/boot", mounts) == nil {
		t.Fatal("expected /boot")
	}
	if lookupMount("/boot/", mounts) == nil {
		t.Fatal("trailing slash should still match")
	}
	if lookupMount("/etc", mounts) != nil {
		t.Fatal("unknown mount")
	}
}

func TestListDirConfined(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := listDirConfined(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("entries %#v", res.Entries)
	}
	if res.Entries[0].Type != "dir" || res.Entries[0].Name != "dir" {
		t.Fatalf("want dir first, got %#v", res.Entries[0])
	}
	sub, err := listDirConfined(root, "dir")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Path != "dir" {
		t.Fatalf("path %q", sub.Path)
	}
	if _, err := listDirConfined(root, ".."); !errors.Is(err, ErrInvalidBrowse) {
		t.Fatalf("traversal: %v", err)
	}
}

func TestUsableMountpoint(t *testing.T) {
	if _, err := usableMountpoint("/z/ds"); err != nil {
		t.Fatal(err)
	}
	for _, mp := range []string{"", "none", "legacy", "-", "relative"} {
		if _, err := usableMountpoint(mp); !errors.Is(err, ErrInvalidBrowse) {
			t.Fatalf("%q: want ErrInvalidBrowse, got %v", mp, err)
		}
	}
}

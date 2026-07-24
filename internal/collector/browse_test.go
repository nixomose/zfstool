package collector

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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

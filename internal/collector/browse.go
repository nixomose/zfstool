package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/zfsname"
)

// ErrInvalidBrowse is returned when a browse path escapes the dataset root or is malformed.
var ErrInvalidBrowse = errors.New("invalid browse path")

const browseMaxEntries = 200

// BrowseDir lists files and directories under a dataset or snapshot mount, confined to that root.
// dataset is a filesystem (pool/fs) or snapshot (pool/fs@snap). relPath is relative to that root ("" = root).
func BrowseDir(ctx context.Context, dataset, relPath string) (*api.BrowseResult, error) {
	if err := zfsname.Check("dataset", dataset); err != nil {
		return nil, err
	}
	root, err := browseRoot(ctx, dataset)
	if err != nil {
		return nil, err
	}
	full, err := confinedJoin(root, relPath)
	if err != nil {
		return nil, err
	}
	st, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%w: not a directory", ErrInvalidBrowse)
	}

	ents, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := &api.BrowseResult{
		Dataset: dataset,
		Path:    normalizeRel(relPath),
		Root:    root,
	}
	type named struct {
		e    api.BrowseEntry
		dir  bool
		name string
	}
	var list []named
	for _, e := range ents {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mode := info.Mode()
		typ := "file"
		switch {
		case mode.IsDir():
			typ = "dir"
		case mode&os.ModeSymlink != 0:
			typ = "symlink"
		case mode&os.ModeNamedPipe != 0:
			typ = "fifo"
		case mode&os.ModeSocket != 0:
			typ = "socket"
		case mode&os.ModeDevice != 0:
			typ = "device"
		}
		be := api.BrowseEntry{Name: name, Type: typ}
		if typ == "file" {
			be.Size = info.Size()
		}
		list = append(list, named{e: be, dir: typ == "dir", name: name})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].dir != list[j].dir {
			return list[i].dir
		}
		return list[i].name < list[j].name
	})
	if len(list) > browseMaxEntries {
		out.Truncated = true
		list = list[:browseMaxEntries]
	}
	out.Entries = make([]api.BrowseEntry, len(list))
	for i := range list {
		out.Entries[i] = list[i].e
	}
	return out, nil
}

func browseRoot(ctx context.Context, dataset string) (string, error) {
	if i := strings.IndexByte(dataset, '@'); i >= 0 {
		parent := dataset[:i]
		snap := dataset[i+1:]
		if parent == "" || snap == "" || strings.Contains(snap, "/") {
			return "", fmt.Errorf("%w: bad snapshot name", ErrInvalidBrowse)
		}
		props, _, err := GetDatasetProperties(ctx, parent)
		if err != nil {
			return "", err
		}
		mp, err := usableMountpoint(props["mountpoint"])
		if err != nil {
			return "", err
		}
		return filepath.Join(mp, ".zfs", "snapshot", snap), nil
	}
	props, _, err := GetDatasetProperties(ctx, dataset)
	if err != nil {
		return "", err
	}
	return usableMountpoint(props["mountpoint"])
}

func usableMountpoint(mp string) (string, error) {
	mp = strings.TrimSpace(mp)
	if mp == "" || mp == "none" || mp == "legacy" || mp == "-" {
		return "", fmt.Errorf("%w: dataset has no absolute mountpoint", ErrInvalidBrowse)
	}
	if !strings.HasPrefix(mp, "/") {
		return "", fmt.Errorf("%w: mountpoint is not absolute", ErrInvalidBrowse)
	}
	return mp, nil
}

func normalizeRel(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return ""
	}
	if strings.HasPrefix(rel, "/") {
		// Absolute client paths are not allowed; treat as invalid via confinedJoin.
		return rel
	}
	return filepath.ToSlash(filepath.Clean(rel))
}

// confinedJoin joins root and a relative path, rejecting traversal and escapes.
func confinedJoin(root, relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if strings.HasPrefix(relPath, "/") {
		return "", fmt.Errorf("%w: path must be relative", ErrInvalidBrowse)
	}
	rel := normalizeRel(relPath)
	if rel != "" {
		for _, p := range strings.Split(rel, "/") {
			if p == ".." || p == "." || p == "" {
				return "", fmt.Errorf("%w: invalid path segment", ErrInvalidBrowse)
			}
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Resolve root (may be a symlink mount); require it exists as a dir.
	rootEval, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// Root missing (e.g. snapdir not visible) — surface as-is for caller.
		if rel == "" {
			return absRoot, nil
		}
		return "", err
	}
	st, err := os.Lstat(rootEval)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%w: browse root is not a directory", ErrInvalidBrowse)
	}

	full := rootEval
	if rel != "" {
		full = filepath.Join(rootEval, filepath.FromSlash(rel))
	}
	// If the target exists, resolve symlinks and ensure still under root.
	if _, err := os.Lstat(full); err == nil {
		eval, err := filepath.EvalSymlinks(full)
		if err != nil {
			return "", err
		}
		full = eval
	}
	relOut, err := filepath.Rel(rootEval, full)
	if err != nil || relOut == ".." || strings.HasPrefix(relOut, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: path escapes dataset root", ErrInvalidBrowse)
	}
	return full, nil
}

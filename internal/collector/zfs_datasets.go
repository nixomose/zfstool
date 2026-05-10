package collector

import (
	"context"
	"strconv"
	"strings"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/execzfs"
)

// ListDatasets lists all datasets with common columns.
// Includes filesystems, snapshots, and volumes. (Plain "zfs list" omits snapshots by default.)
func ListDatasets(ctx context.Context, pool string) ([]api.DatasetRow, error) {
	args := []string{
		"list", "-Hp",
		"-t", "filesystem,snapshot,volume",
		"-o", "name,type,used,avail,refer,mountpoint,origin",
	}
	if pool != "" {
		args = append(args, "-r", pool)
	}
	out, err := execzfs.RunZfs(ctx, args...)
	if err != nil {
		return nil, err
	}
	var rows []api.DatasetRow
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 5 {
			continue
		}
		r := api.DatasetRow{Name: f[0], Type: f[1]}
		if v, e := strconv.ParseUint(f[2], 10, 64); e == nil {
			r.Used = v
		}
		if v, e := strconv.ParseUint(f[3], 10, 64); e == nil {
			r.Avail = v
		}
		if v, e := strconv.ParseUint(f[4], 10, 64); e == nil {
			r.Refer = v
		}
		if len(f) > 5 {
			r.Mountpoint = f[5]
		}
		if len(f) > 6 {
			r.Origin = f[6]
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// GetDatasetProperties runs zfs get -H -p -s local,default,inherited,received -o name,property,value,source all dataset
func GetDatasetProperties(ctx context.Context, name string) (map[string]string, map[string]string, error) {
	out, err := execzfs.RunZfs(ctx, "get", "-H", "-p", "-o", "property,value,source", "all", name)
	if err != nil {
		return nil, nil, err
	}
	props := make(map[string]string)
	src := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		k := parts[0]
		props[k] = parts[1]
		if len(parts) > 2 {
			src[k] = parts[2]
		}
	}
	return props, src, nil
}

// ListBookmarks lists bookmarks (-t bookmark).
func ListBookmarks(ctx context.Context, pool string) ([]api.DatasetRow, error) {
	args := []string{"list", "-Hp", "-t", "bookmark", "-o", "name"}
	if pool != "" {
		args = append(args, "-r", pool)
	}
	out, err := execzfs.RunZfs(ctx, args...)
	if err != nil {
		return nil, err
	}
	var rows []api.DatasetRow
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 1 {
			continue
		}
		rows = append(rows, api.DatasetRow{Name: f[0], Type: "bookmark"})
	}
	return rows, nil
}

// Holds returns zfs holds for a snapshot.
func Holds(ctx context.Context, snap string) (map[string]string, error) {
	out, err := execzfs.RunZfs(ctx, "holds", "-H", snap)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) >= 2 {
			m[f[1]] = f[0] // tag -> timestamp rough
		}
	}
	return m, nil
}

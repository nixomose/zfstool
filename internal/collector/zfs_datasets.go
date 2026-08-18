package collector

import (
	"context"
	"strconv"
	"strings"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/execzfs"
	"github.com/nixomose/zfstool/internal/zfsname"
)

const datasetListColsFull = "name,type,used,avail,refer,mountpoint,origin,usedbydataset,usedbysnapshots,usedbychildren,usedbyrefreservation"
const datasetListColsBasic = "name,type,used,avail,refer,mountpoint,origin"

// ListDatasets lists all datasets with common columns.
// Includes filesystems, snapshots, and volumes. (Plain "zfs list" omits snapshots by default.)
func ListDatasets(ctx context.Context, pool string) ([]api.DatasetRow, error) {
	rows, err := listDatasetsWith(ctx, pool, datasetListColsFull)
	if err != nil {
		return listDatasetsWith(ctx, pool, datasetListColsBasic)
	}
	return rows, nil
}

func listDatasetsWith(ctx context.Context, pool, cols string) ([]api.DatasetRow, error) {
	args := []string{
		"list", "-Hp",
		"-t", "filesystem,snapshot,volume",
		"-o", cols,
	}
	if pool != "" {
		var err error
		args, err = zfsname.Append(append(args, "-r"), "pool", pool)
		if err != nil {
			return nil, err
		}
	}
	out, err := execzfs.RunZfs(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseDatasetList(out), nil
}

func parseDatasetList(out []byte) []api.DatasetRow {
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
		r.Used = parseZfsUint(f[2])
		r.Avail = parseZfsUint(f[3])
		r.Refer = parseZfsUint(f[4])
		if len(f) > 5 && f[5] != "-" {
			r.Mountpoint = f[5]
		}
		if len(f) > 6 && f[6] != "-" {
			r.Origin = f[6]
		}
		if len(f) > 7 {
			r.UsedByDataset = parseZfsUint(f[7])
		}
		if len(f) > 8 {
			r.UsedBySnapshots = parseZfsUint(f[8])
		}
		if len(f) > 9 {
			r.UsedByChildren = parseZfsUint(f[9])
		}
		if len(f) > 10 {
			r.UsedByRefreservation = parseZfsUint(f[10])
		}
		rows = append(rows, r)
	}
	return rows
}

func parseZfsUint(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// GetDatasetProperties runs zfs get -H -p -o property,value,source all dataset
func GetDatasetProperties(ctx context.Context, name string) (map[string]string, map[string]string, error) {
	args, err := zfsname.Append([]string{"get", "-H", "-p", "-o", "property,value,source", "all"}, "dataset", name)
	if err != nil {
		return nil, nil, err
	}
	out, err := execzfs.RunZfs(ctx, args...)
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
		var err error
		args, err = zfsname.Append(append(args, "-r"), "pool", pool)
		if err != nil {
			return nil, err
		}
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
	args, err := zfsname.Append([]string{"holds", "-H"}, "snapshot", snap)
	if err != nil {
		return nil, err
	}
	out, err := execzfs.RunZfs(ctx, args...)
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

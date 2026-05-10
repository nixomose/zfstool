package collector

import (
	"context"
	"strconv"
	"strings"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/execzfs"
)

// PoolHistory runs zpool history -l pool with optional tail via offset/limit (post-filter).
func PoolHistory(ctx context.Context, pool string, offset, limit int) ([]api.PoolHistoryEntry, error) {
	out, err := execzfs.RunZpool(ctx, "history", "-l", pool)
	if err != nil {
		return nil, err
	}
	var all []api.PoolHistoryEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "History for") {
			continue
		}
		// Format: 2024-01-02.12:00:00 command...
		idx := strings.Index(line, " ")
		if idx <= 0 {
			continue
		}
		ts := line[:idx]
		cmd := strings.TrimSpace(line[idx:])
		all = append(all, api.PoolHistoryEntry{Time: ts, Command: cmd})
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(all) {
		return []api.PoolHistoryEntry{}, nil
	}
	end := len(all)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return all[offset:end], nil
}

// PoolProperties runs zpool get all pool -> map
func PoolProperties(ctx context.Context, pool string) (map[string]string, map[string]string, error) {
	out, err := execzfs.RunZpool(ctx, "get", "-H", "-p", "-o", "property,value,source", "all", pool)
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
		props[parts[0]] = parts[1]
		if len(parts) > 2 {
			src[parts[0]] = parts[2]
		}
	}
	return props, src, nil
}

// Maintenance collects autotrim + scan from status + props.
func Maintenance(ctx context.Context, pool string) (*api.MaintenanceBundle, error) {
	st, err := PoolStatusFull(ctx, pool)
	if err != nil {
		return nil, err
	}
	props, _, err := PoolProperties(ctx, pool)
	if err != nil {
		props = map[string]string{}
	}
	mb := &api.MaintenanceBundle{
		Pool:     pool,
		Autotrim: props["autotrim"],
		Scan:     st.Scan,
		Props:    map[string]string{},
	}
	for _, k := range []string{"autotrim", "scrub_expanded", "health"} {
		if v, ok := props[k]; ok {
			mb.Props[k] = v
		}
	}
	return mb, nil
}

// IOStatOneShot runs zpool iostat -Hp 1 1 (one sample).
func IOStatOneShot(ctx context.Context) ([]api.IOStatSample, error) {
	out, err := execzfs.RunZpool(ctx, "iostat", "-Hp", "1", "2")
	if err != nil {
		return nil, err
	}
	var samples []api.IOStatSample
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		s := api.IOStatSample{Raw: f}
		if len(f) > 0 {
			s.Pool = f[0]
		}
		if len(f) > 1 {
			s.VDev = f[1]
		}
		if len(f) > 2 {
			s.CapOpsRead, _ = strconv.ParseFloat(f[2], 64)
		}
		if len(f) > 3 {
			s.CapOpsWrite, _ = strconv.ParseFloat(f[3], 64)
		}
		if len(f) > 4 {
			s.BandwidthRead = f[4]
		}
		if len(f) > 5 {
			s.BandwidthWrite = f[5]
		}
		samples = append(samples, s)
	}
	return samples, nil
}

package collector

import (
	"context"
	"strconv"
	"strings"

	"zfstool/internal/api"
	"zfstool/internal/execzfs"
)

// ListPools runs zpool list -Hp -o ...
func ListPools(ctx context.Context) ([]api.PoolSummary, error) {
	// name size allocated free fragmentation capacity health dedupratio altroot
	out, err := execzfs.RunZpool(ctx, "list", "-Hp", "-o", "name,size,allocated,free,fragmentation,capacity,health,dedupratio,altroot")
	if err != nil {
		return nil, err
	}
	var pools []api.PoolSummary
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		p := api.PoolSummary{Name: fields[0]}
		if v, e := strconv.ParseUint(fields[1], 10, 64); e == nil {
			p.Size = v
		}
		if v, e := strconv.ParseUint(fields[2], 10, 64); e == nil {
			p.Allocated = v
		}
		if v, e := strconv.ParseUint(fields[3], 10, 64); e == nil {
			p.Free = v
		}
		if frag, e := parseMaybePercent(fields[4]); e == nil && frag != nil {
			p.Fragmentation = frag
		}
		if capv, e := parseMaybePercent(fields[5]); e == nil && capv != nil {
			p.CapacityPct = capv
		}
		p.Health = fields[6]
		if len(fields) > 7 {
			p.DedupRatio = fields[7]
		}
		if len(fields) > 8 {
			p.AltRoot = fields[8]
		}
		pools = append(pools, p)
	}
	return pools, nil
}

func parseMaybePercent(s string) (*uint64, error) {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return nil, nil
	}
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

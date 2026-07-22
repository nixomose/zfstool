package collector

import (
	"context"
	"sort"

	"github.com/nixomose/zfstool/internal/api"
)

// ListDisks aggregates unique block devices from all pool status trees.
func ListDisks(ctx context.Context) ([]api.DiskSummary, error) {
	pools, err := ListPools(ctx)
	if err != nil {
		return nil, err
	}
	byDev := map[string]*api.DiskSummary{}
	for _, p := range pools {
		st, err := PoolStatusFull(ctx, p.Name)
		if err != nil {
			continue
		}
		enriched := enrichStatusLines(st.Config, p.Name)
		for _, ln := range enriched {
			if !ln.isDisk {
				continue
			}
			path := normalizeDiskPath(ln.name)
			d, ok := byDev[path]
			if !ok {
				d = &api.DiskSummary{Device: path}
				byDev[path] = d
			}
			d.Pools = append(d.Pools, api.DiskMembership{
				Pool:  p.Name,
				State: ln.state,
				Path:  ln.vdevPath,
			})
		}
	}
	out := make([]api.DiskSummary, 0, len(byDev))
	for _, d := range byDev {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Device < out[j].Device
	})
	return out, nil
}

type statusLineEx struct {
	name     string
	state    string
	indent   int
	isDisk   bool
	vdevPath string
}

func enrichStatusLines(config []api.StatusLine, poolName string) []statusLineEx {
	stack := []struct {
		indent int
		name   string
	}{}
	out := make([]statusLineEx, 0, len(config))
	for _, ln := range config {
		for len(stack) > 0 && stack[len(stack)-1].indent >= ln.Indent {
			stack = stack[:len(stack)-1]
		}
		ancestors := make([]string, len(stack))
		for i, s := range stack {
			ancestors[i] = s.name
		}
		stack = append(stack, struct {
			indent int
			name   string
		}{ln.Indent, ln.Name})
		vdevPath := poolName
		if len(ancestors) > 0 {
			vdevPath = ""
			for i, a := range ancestors {
				if i > 0 {
					vdevPath += " › "
				}
				vdevPath += a
			}
		}
		out = append(out, statusLineEx{
			name:     ln.Name,
			state:    ln.State,
			indent:   ln.Indent,
			isDisk:   isDiskLeafName(ln.Name),
			vdevPath: vdevPath,
		})
	}
	return out
}

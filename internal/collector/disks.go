package collector

import "github.com/nixomose/zfstool/internal/api"

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

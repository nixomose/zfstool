package collector

import (
	"context"
	"strings"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/execzfs"
	"github.com/nixomose/zfstool/internal/zfsname"
)

// PoolStatusFull returns parsed status + raw text.
// Uses zpool status -P so the config tree lists full device paths (/dev/...).
// (-g shows vdev GUIDs, which are not useful for disk counts or SMART paths.)
func PoolStatusFull(ctx context.Context, pool string) (*api.PoolStatus, error) {
	args, err := zfsname.Append([]string{"status", "-P"}, "pool", pool)
	if err != nil {
		return nil, err
	}
	out, err := execzfs.RunZpool(ctx, args...)
	if err != nil {
		return nil, err
	}
	raw := string(out)
	st := parseStatus(raw)
	st.Pool = pool
	st.Raw = raw
	return st, nil
}

func parseStatus(raw string) *api.PoolStatus {
	st := &api.PoolStatus{}
	lines := strings.Split(raw, "\n")
	var inConfig bool
	var scanLines []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "state:") {
			st.State = strings.TrimSpace(strings.TrimPrefix(trim, "state:"))
			continue
		}
		if strings.HasPrefix(trim, "scan:") {
			scanLines = append(scanLines, strings.TrimSpace(strings.TrimPrefix(trim, "scan:")))
			continue
		}
		if strings.HasPrefix(trim, "scrub:") {
			scanLines = append(scanLines, strings.TrimSpace(strings.TrimPrefix(trim, "scrub:")))
			continue
		}
		if strings.HasPrefix(trim, "resilver:") {
			scanLines = append(scanLines, "resilver: "+strings.TrimSpace(strings.TrimPrefix(trim, "resilver:")))
			continue
		}
		if strings.Contains(strings.ToLower(trim), "checkpoint") {
			st.Checkpoint = parseCheckpointLine(trim)
			continue
		}
		if trim == "config:" {
			inConfig = true
			continue
		}
		if inConfig {
			if trim == "" {
				continue
			}
			if strings.HasPrefix(trim, "errors:") {
				inConfig = false
				st.Errors = strings.TrimSpace(strings.TrimPrefix(trim, "errors:"))
				continue
			}
			if strings.HasPrefix(trim, "NAME") {
				continue
			}
			if strings.HasPrefix(trim, "├") || strings.HasPrefix(trim, "└") {
				continue
			}
			if sl := parseStatusConfigLine(line); sl != nil {
				st.Config = append(st.Config, *sl)
			}
		}
		if strings.HasPrefix(trim, "errors:") {
			st.Errors = strings.TrimSpace(strings.TrimPrefix(trim, "errors:"))
		}
	}
	if len(scanLines) > 0 {
		combined := strings.Join(scanLines, "; ")
		st.ScanRaw = combined
		st.Scan = parseScanLine(combined)
	}
	return st
}

func parseCheckpointLine(line string) *api.Checkpoint {
	c := &api.Checkpoint{Active: true, Detail: line}
	if strings.Contains(strings.ToLower(line), "no checkpoint") || strings.Contains(strings.ToLower(line), "none") {
		c.Active = false
	}
	// "checkpoint: name@..."
	lower := strings.ToLower(line)
	if idx := strings.Index(lower, "checkpoint"); idx >= 0 {
		rest := strings.TrimSpace(line[idx+len("checkpoint"):])
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimSpace(rest)
		c.Name = rest
	}
	return c
}

func parseStatusConfigLine(line string) *api.StatusLine {
	// zpool status uses leading spaces for indent; columns tab or multi-space
	orig := line
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return nil
	}
	indent := 0
	for i, r := range line {
		if r == ' ' || r == '\t' {
			if r == '\t' {
				indent += 8
			} else {
				indent++
			}
			continue
		}
		line = line[i:]
		break
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	// NAME STATE READ WRITE CKSUM
	if fields[0] == "NAME" {
		return nil
	}
	sl := &api.StatusLine{Indent: indent, Name: fields[0]}
	if len(fields) > 1 {
		sl.State = fields[1]
	}
	if len(fields) > 2 {
		sl.Read = fields[2]
	}
	if len(fields) > 3 {
		sl.Write = fields[3]
	}
	if len(fields) > 4 {
		sl.Cksum = fields[4]
	}
	_ = orig
	return sl
}

func parseScanLine(s string) *api.ScanInfo {
	si := &api.ScanInfo{Raw: s}
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "resilver"):
		si.Type = "resilver"
	case strings.Contains(low, "scrub"):
		si.Type = "scrub"
	case strings.Contains(low, "trim"):
		si.Type = "trim"
	default:
		si.Type = "unknown"
	}
	if strings.Contains(low, "in progress") || strings.Contains(low, "progress:") {
		si.InProgress = true
	}
	if strings.Contains(low, "repaired") || strings.Contains(low, "errors") {
		si.Errors = s
	}
	// last scrub completed / resilver done patterns
	if strings.Contains(low, "completed") {
		si.LastRun = s
	}
	si.Progress = s
	return si
}

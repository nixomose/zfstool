package collector

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// KernelLogSnippet returns recent kernel lines mentioning zfs or pool.
func KernelLogSnippet(ctx context.Context, pool string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		maxLines = 50
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// journalctl -k -n 200 --no-pager
	cmd := exec.CommandContext(ctx, "journalctl", "-k", "-n", "200", "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		// fallback: dmesg tail
		b, e2 := exec.CommandContext(ctx, "dmesg", "-T").Output()
		if e2 != nil {
			return nil, err
		}
		out = b
	}
	var lines []string
	lowPool := strings.ToLower(pool)
	for _, ln := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(ln)
		if l == "" {
			continue
		}
		ll := strings.ToLower(l)
		if strings.Contains(ll, "zfs") || (lowPool != "" && strings.Contains(ll, lowPool)) {
			lines = append(lines, l)
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

// ModuleParams lists readable zfs module parameters (names only + values).
func ModuleParams() map[string]string {
	m := make(map[string]string)
	dir := "/sys/module/zfs/parameters"
	ents, err := os.ReadDir(dir)
	if err != nil {
		return m
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		m[e.Name()] = strings.TrimSpace(string(b))
	}
	return m
}

// ZfsAllowOutput runs zfs allow dataset (may fail without permission).
func ZfsAllowOutput(ctx context.Context, dataset string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zfs", "allow", dataset)
	out, err := cmd.Output()
	return string(out), err
}

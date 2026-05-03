package collector

import (
	"context"
	"strings"

	"zfstool/internal/execzfs"
)

// ZfsDiff runs zfs diff -H from to (snapshots or dataset versions).
func ZfsDiff(ctx context.Context, from, to string) (string, error) {
	out, err := execzfs.RunZfs(ctx, "diff", "-H", from, to)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

package collector

import (
	"context"
	"strings"

	"github.com/nixomose/zfstool/internal/execzfs"
	"github.com/nixomose/zfstool/internal/zfsname"
)

// ZfsDiff runs zfs diff -H from to (snapshots or dataset versions).
func ZfsDiff(ctx context.Context, from, to string) (string, error) {
	args, err := zfsname.Append2([]string{"diff", "-H"}, "from", from, "to", to)
	if err != nil {
		return "", err
	}
	out, err := execzfs.RunZfs(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

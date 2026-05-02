package collector

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"zfstool/internal/api"
)

// SMARTJSON runs smartctl -a -j device (best effort).
func SMARTJSON(ctx context.Context, device string) (*api.SMARTDisk, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "smartctl", "-a", "-j", device)
	out, err := cmd.Output()
	d := &api.SMARTDisk{Device: device}
	if err != nil {
		d.Error = err.Error()
		return d, nil
	}
	var m map[string]interface{}
	if json.Unmarshal(out, &m) == nil {
		d.JSON = m
	}
	return d, nil
}

// MapVDevToDevices extracts device paths from status config lines (leaf names often /dev/...).
func MapVDevToDevices(config []api.StatusLine) []string {
	var devs []string
	seen := map[string]struct{}{}
	for _, ln := range config {
		n := ln.Name
		if strings.HasPrefix(n, "/dev/") {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				devs = append(devs, n)
			}
		}
	}
	return devs
}

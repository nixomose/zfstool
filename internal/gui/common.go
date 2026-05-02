package gui

import "os"

func defaultSocket() string {
	if p := os.Getenv("ZFSTOOL_SOCKET"); p != "" {
		return p
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d + "/zfstool/agent.sock"
	}
	return "/run/zfstool/agent.sock"
}

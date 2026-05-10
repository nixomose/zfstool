package gui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nixomose/zfstool/internal/agent"
)

// embeddedAgentSocketPath returns a per-process private socket path for an in-process agent.
func embeddedAgentSocketPath() string {
	base := filepath.Join(os.TempDir(), "zfstool-embed")
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		base = filepath.Join(d, "zfstool", "embed")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return filepath.Join(os.TempDir(), fmt.Sprintf("zfstool-agent-%d.sock", os.Getpid()))
	}
	return filepath.Join(base, fmt.Sprintf("agent-%d.sock", os.Getpid()))
}

// startEmbeddedAgent runs the read-only API agent in the background on a private Unix socket.
// stop is idempotent and shuts down the listener and removes the socket.
func startEmbeddedAgent() (socketPath string, stop func(), err error) {
	socketPath = embeddedAgentSocketPath()
	ag := agent.NewServer()
	ag.SocketPath = socketPath
	ln, err := ag.ListenUnix()
	if err != nil {
		return "", nil, err
	}
	hs := &http.Server{Handler: ag}
	var once sync.Once
	stop = func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = hs.Shutdown(ctx)
			_ = ln.Close()
			_ = os.Remove(socketPath)
		})
	}
	go func() {
		if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("embedded agent: %v", err)
		}
	}()
	// Let the serve goroutine call Accept before clients dial (e.g. GTK loads data immediately).
	time.Sleep(50 * time.Millisecond)
	log.Printf("zfstool embedded agent listening unix:%s", socketPath)
	return socketPath, stop, nil
}

// resolveGUIAgent picks how the GUI talks to an agent:
//   - agentURL non-empty: remote HTTP agent (no embed).
//   - agent-socket flag non-empty: that Unix socket (no embed).
//   - otherwise: always an embedded agent on a private socket.
//
// ZFSTOOL_SOCKET is intentionally not used here so a plain "zfstool" / "zfstool gui"
// is self-contained. Use -agent-socket to point at a systemd or manual agent.
func resolveGUIAgent(agentSocketFlag, agentURLFlag string) (unixSock string, httpAgent string, stop func(), err error) {
	noop := func() {}
	if agentURLFlag != "" {
		return "", agentURLFlag, noop, nil
	}
	if agentSocketFlag != "" {
		return agentSocketFlag, "", noop, nil
	}
	sock, st, e := startEmbeddedAgent()
	if e != nil {
		return "", "", nil, e
	}
	return sock, "", st, nil
}

package gui

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"zfstool/internal/web"
)

// LocalSession is the loopback HTTP server (static UI + /v1 proxy) plus agent lifecycle.
type LocalSession struct {
	BaseURL string

	embedded  bool
	agentUnix string
	agentHTTP string

	httpServer *http.Server
	stopAgent  func()
}

// Embedded reports whether the agent was started in-process for this session.
func (s *LocalSession) Embedded() bool { return s.embedded }

// BackendDescription is the Unix path or HTTP URL of the agent this UI talks to.
func (s *LocalSession) BackendDescription() string {
	if s.agentHTTP != "" {
		return s.agentHTTP
	}
	return s.agentUnix
}

// Stop shuts down the UI server and the embedded agent (if any).
func (s *LocalSession) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.httpServer != nil {
		_ = s.httpServer.Shutdown(ctx)
	}
	if s.stopAgent != nil {
		s.stopAgent()
	}
}

// StartLocalSession starts an embedded agent when no external agent is configured,
// then serves the bundled web UI and proxies /v1 to that agent on 127.0.0.1:0.
func StartLocalSession(agentSockFlag, agentURLFlag string) (*LocalSession, error) {
	unixSock, httpAgentURL, stopAgent, err := resolveGUIAgent(agentSockFlag, agentURLFlag)
	if err != nil {
		return nil, err
	}
	embedded := agentSockFlag == "" && agentURLFlag == ""

	var srv *web.Server
	if httpAgentURL != "" {
		srv, err = web.NewServerWithHTTPAgent("", httpAgentURL, "")
	} else {
		srv, err = web.NewServer("", unixSock, "")
	}
	if err != nil {
		stopAgent()
		return nil, err
	}
	sub, err := web.StaticSubFS()
	if err != nil {
		stopAgent()
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/", http.HandlerFunc(srv.ServeProxy))
	mux.Handle("/", http.FileServer(http.FS(sub)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		stopAgent()
		return nil, err
	}
	baseURL := "http://" + ln.Addr().String() + "/"

	ui := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		if err := ui.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("zfstool local UI server: %v", err)
		}
	}()

	sess := &LocalSession{
		BaseURL:    baseURL,
		embedded:   embedded,
		agentUnix:  unixSock,
		agentHTTP:  httpAgentURL,
		httpServer: ui,
		stopAgent:  stopAgent,
	}
	log.Printf("zfstool local UI %s (agent %s, embedded=%v)", baseURL, sess.BackendDescription(), embedded)
	return sess, nil
}

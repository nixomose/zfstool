package web

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// StaticSubFS returns embedded UI files for the gui subcommand.
func StaticSubFS() (fs.FS, error) {
	return fs.Sub(static, "static")
}

//go:embed static/*
var static embed.FS

// Server is the zfstool web front-end (proxies /v1 to agent + static UI).
type Server struct {
	ListenAddr  string
	AgentSocket string
	PAMService  string
	proxy       *httputil.ReverseProxy
}

// Run parses flags and blocks serving.
func Run(args []string) {
	flg := flag.NewFlagSet("web", flag.ExitOnError)
	listen := flg.String("listen", "127.0.0.1:8787", "HTTP listen address")
	socket := flg.String("agent-socket", defaultAgentSocket(), "agent unix socket")
	pamSvc := flg.String("pam-service", "login", "PAM service (unused unless built with pam)")
	_ = flg.Parse(args)

	s, err := NewServer(*listen, *socket, *pamSvc)
	if err != nil {
		log.Fatal(err)
	}
	sub, err := StaticSubFS()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/", s.authMiddleware(http.HandlerFunc(s.proxyAgent)))
	mux.Handle("/", s.authMiddleware(http.FileServer(http.FS(sub))))

	log.Printf("zfstool web listening %s (proxy /v1 -> unix:%s)", s.ListenAddr, s.AgentSocket)
	_ = pamSvc
	log.Fatal(http.ListenAndServe(s.ListenAddr, mux))
}

func defaultAgentSocket() string {
	if p := os.Getenv("ZFSTOOL_SOCKET"); p != "" {
		return p
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d + "/zfstool/agent.sock"
	}
	return "/run/zfstool/agent.sock"
}

// NewServer builds reverse proxy to agent unix socket.
func NewServer(listen, agentSocket, pamService string) (*Server, error) {
	s := &Server{ListenAddr: listen, AgentSocket: agentSocket, PAMService: pamService}
	u, _ := url.Parse("http://localhost")
	_ = u
	s.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost"
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", s.AgentSocket)
			},
		},
	}
	return s, nil
}

func (s *Server) proxyAgent(w http.ResponseWriter, r *http.Request) {
	s.proxy.ServeHTTP(w, r)
}

// ServeProxy forwards /v1 to the agent (same as proxyAgent, exported).
func (s *Server) ServeProxy(w http.ResponseWriter, r *http.Request) {
	s.proxy.ServeHTTP(w, r)
}

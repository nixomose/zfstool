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
	ListenAddr     string
	AgentSocket    string // unix path when AgentHTTPURL is empty
	AgentHTTPURL   string // optional e.g. http://127.0.0.1:8787 (TCP agent)
	PAMService     string
	proxy          *httputil.ReverseProxy
}

// Run parses flags and blocks serving.
func Run(args []string) {
	flg := flag.NewFlagSet("web", flag.ExitOnError)
	listen := flg.String("listen", "127.0.0.1:8787", "HTTP listen address")
	socket := flg.String("agent-socket", defaultAgentSocket(), "agent unix socket")
	pamSvc := flg.String("pam-service", "login", "PAM service (unused unless built with pam)")
	_ = flg.Parse(args)

	s, err := newServerBackend(*listen, *socket, "", *pamSvc)
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

	if s.AgentHTTPURL != "" {
		log.Printf("zfstool web listening %s (proxy /v1 -> %s)", s.ListenAddr, s.AgentHTTPURL)
	} else {
		log.Printf("zfstool web listening %s (proxy /v1 -> unix:%s)", s.ListenAddr, s.AgentSocket)
	}
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

// NewServer builds reverse proxy to the agent on a Unix domain socket.
func NewServer(listen, agentSocket, pamService string) (*Server, error) {
	return newServerBackend(listen, agentSocket, "", pamService)
}

// NewServerWithHTTPAgent proxies /v1 to an agent listening on agentHTTPURL (e.g. http://host:8787).
func NewServerWithHTTPAgent(listen, agentHTTPURL, pamService string) (*Server, error) {
	return newServerBackend(listen, "", agentHTTPURL, pamService)
}

func newServerBackend(listen, agentUnix, agentHTTP, pamService string) (*Server, error) {
	s := &Server{
		ListenAddr:   listen,
		AgentSocket:  agentUnix,
		AgentHTTPURL: agentHTTP,
		PAMService:   pamService,
	}
	switch {
	case agentHTTP != "":
		u, err := url.Parse(agentHTTP)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, os.ErrInvalid
		}
		s.proxy = &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
			},
		}
	case agentUnix != "":
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
	default:
		return nil, os.ErrInvalid
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

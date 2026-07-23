package agent

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// Server serves the zfstool read-only HTTP API over a Unix domain socket (or TCP for tests).
type Server struct {
	SocketPath string
	HTTPAddr   string // if set, also listen on TCP (e.g. 127.0.0.1:8787)
	mux        *http.ServeMux
}

// NewServer builds default routes.
func NewServer() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /v1/version", s.handleVersion)
	m.HandleFunc("GET /v1/host", s.handleHost)
	m.HandleFunc("GET /v1/pools", s.handlePools)
	m.HandleFunc("GET /v1/pools/{pool}/status", s.handlePoolStatus)
	m.HandleFunc("GET /v1/pools/{pool}/properties", s.handlePoolProps)
	m.HandleFunc("GET /v1/pools/{pool}/history", s.handlePoolHistory)
	m.HandleFunc("GET /v1/pools/{pool}/maintenance", s.handlePoolMaintenance)
	m.HandleFunc("GET /v1/pools/{pool}/devices", s.handlePoolDevices)
	m.HandleFunc("GET /v1/datasets", s.handleDatasets)
	m.HandleFunc("GET /v1/datasets/properties", s.handleDatasetProps)
	m.HandleFunc("GET /v1/bookmarks", s.handleBookmarks)
	m.HandleFunc("GET /v1/snapshots/holds", s.handleHolds)
	m.HandleFunc("GET /v1/iostat", s.handleIOStat)
	m.HandleFunc("GET /v1/graph", s.handleGraph)
	m.HandleFunc("GET /v1/kernel-log", s.handleKernelLog)
	m.HandleFunc("GET /v1/module-params", s.handleModuleParams)
	m.HandleFunc("GET /v1/zfs-allow", s.handleZfsAllow)
	m.HandleFunc("GET /v1/disks", s.handleDisks)
	m.HandleFunc("GET /v1/disk/{dev}/smart", s.handleSmart)
	m.HandleFunc("POST /v1/zfs-diff", s.handleZfsDiff)
}

// ListenUnix creates parent dirs, binds a Unix socket at SocketPath, and returns the listener.
func (s *Server) ListenUnix() (net.Listener, error) {
	if s.SocketPath == "" {
		return nil, os.ErrInvalid
	}
	dir := filepath.Dir(s.SocketPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(s.SocketPath)
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(s.SocketPath, 0o600); err != nil {
		log.Printf("chmod socket: %v", err)
	}
	return ln, nil
}

// Serve handles HTTP on ln until the listener is closed.
func (s *Server) Serve(ln net.Listener) error {
	return http.Serve(ln, s.mux)
}

// ListenAndServeUnix creates parent dir, listens on SocketPath, serves until error.
func (s *Server) ListenAndServeUnix() error {
	ln, err := s.ListenUnix()
	if err != nil {
		return err
	}
	log.Printf("zfstool server listening unix:%s", s.SocketPath)
	return s.Serve(ln)
}

// ListenAndServeTCP listens on HTTPAddr (e.g. 127.0.0.1:8787).
func (s *Server) ListenAndServeTCP() error {
	if s.HTTPAddr == "" {
		return os.ErrInvalid
	}
	log.Printf("zfstool server listening tcp:%s", s.HTTPAddr)
	return http.ListenAndServe(s.HTTPAddr, s.mux)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

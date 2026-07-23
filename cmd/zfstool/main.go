package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/nixomose/zfstool/internal/agent"
	"github.com/nixomose/zfstool/internal/gui"
	"github.com/nixomose/zfstool/internal/version"
	"github.com/nixomose/zfstool/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		gui.Run(nil)
		return
	}
	first := os.Args[1]
	if strings.HasPrefix(first, "-") {
		switch first {
		case "-h", "--help":
			usage()
			return
		case "-v", "--version":
			fmt.Println(version.Version)
			return
		default:
			// e.g. zfstool -agent-socket=... or zfstool --no-browser
			gui.Run(os.Args[1:])
			return
		}
	}
	switch first {
	case "server", "agent": // "agent" kept as alias
		runServer(os.Args[2:])
	case "web":
		web.Run(os.Args[2:])
	case "gui":
		gui.Run(os.Args[2:])
	case "version":
		fmt.Println(version.Version)
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", first)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  zfstool [gui flags]     open the desktop app (WebKit window + in-process server; else browser)
  zfstool gui [flags]   same as above, explicit
  zfstool server [flags]  run the API server (Unix socket)
  zfstool web [flags]   run the HTTP front-end + web UI (proxies to the API server)
  zfstool version       print version
  zfstool help          show this message

gui flags: -agent-url for a remote HTTP API; -agent-socket for an external Unix API server.
If neither is set, an embedded API server runs in-process. ZFSTOOL_SOCKET is not used for the GUI.

Typical browser setup (two processes on the ZFS host):
  zfstool server
  zfstool web
  then open http://127.0.0.1:8787/
`)
}

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	socket := fs.String("socket", defaultSocket(), "unix socket path")
	httpAddr := fs.String("http", "", "optional TCP listen address (e.g. 127.0.0.1:8787); unauthenticated — prefer Unix socket")
	allowRemote := fs.Bool("allow-remote-http", false, "allow -http to bind a non-loopback address (dangerous: no auth)")
	_ = fs.Parse(args)

	if *httpAddr != "" {
		host, _, err := net.SplitHostPort(*httpAddr)
		if err != nil {
			log.Fatalf("invalid -http address %q: %v", *httpAddr, err)
		}
		ip := net.ParseIP(host)
		loopback := host == "localhost" || (ip != nil && ip.IsLoopback())
		if !loopback && !*allowRemote {
			log.Fatalf("refusing non-loopback -http %q without -allow-remote-http (API server has no authentication)", *httpAddr)
		}
		if !loopback {
			log.Printf("WARNING: server TCP on %s is unauthenticated; anyone who can reach it can read ZFS/SMART/logs", *httpAddr)
		} else {
			log.Printf("WARNING: server TCP on %s is unauthenticated; prefer Unix socket for production", *httpAddr)
		}
	}

	srv := agent.NewServer()
	srv.SocketPath = *socket
	srv.HTTPAddr = *httpAddr

	if srv.HTTPAddr != "" {
		go func() {
			if err := srv.ListenAndServeTCP(); err != nil {
				log.Fatal(err)
			}
		}()
	}
	if err := srv.ListenAndServeUnix(); err != nil {
		log.Fatal(err)
	}
}

func defaultSocket() string {
	if p := os.Getenv("ZFSTOOL_SOCKET"); p != "" {
		return p
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d + "/zfstool/agent.sock"
	}
	return "/run/zfstool/agent.sock"
}

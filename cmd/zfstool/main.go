package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"zfstool/internal/agent"
	"zfstool/internal/gui"
	"zfstool/internal/version"
	"zfstool/internal/web"
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
	case "agent":
		runAgent(os.Args[2:])
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
  zfstool [gui flags]     open the desktop UI (GTK when built with -tags gtk3, else browser)
  zfstool gui [flags]   same as above, explicit
  zfstool agent [flags] run the API agent (Unix socket)
  zfstool web [flags]   run the HTTP server + web UI
  zfstool version       print version
  zfstool help          show this message
`)
}

func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	socket := fs.String("socket", defaultSocket(), "unix socket path")
	httpAddr := fs.String("http", "", "optional TCP listen address (e.g. 127.0.0.1:8787)")
	_ = fs.Parse(args)

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

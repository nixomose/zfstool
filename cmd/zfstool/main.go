package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"zfstool/internal/agent"
	"zfstool/internal/gui"
	"zfstool/internal/version"
	"zfstool/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "agent":
		runAgent(os.Args[2:])
	case "web":
		web.Run(os.Args[2:])
	case "gui":
		gui.Run(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(version.Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: zfstool <agent|web|gui|version> [flags]\n")
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

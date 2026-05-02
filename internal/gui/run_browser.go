//go:build !gtk3

package gui

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"

	"zfstool/internal/web"
)

// Run starts a local loopback server with static UI + proxy to agent, opens browser.
func Run(args []string) {
	flg := flag.NewFlagSet("gui", flag.ExitOnError)
	socket := flg.String("agent-socket", defaultSocket(), "zfstool agent unix socket")
	noBrowser := flg.Bool("no-browser", false, "do not open a browser window")
	_ = flg.Parse(args)

	srv, err := web.NewServer("", *socket, "")
	if err != nil {
		log.Fatal(err)
	}
	sub, err := web.StaticSubFS()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/", http.HandlerFunc(srv.ServeProxy))
	mux.Handle("/", http.FileServer(http.FS(sub)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	addr := ln.Addr().String()
	page := "http://" + addr + "/"

	if !*noBrowser {
		go openBrowser(page)
	}
	log.Printf("zfstool gui %s (agent unix:%s)", page, *socket)
	log.Fatal(http.Serve(ln, mux))
}

func openBrowser(url string) {
	time.Sleep(200 * time.Millisecond)
	for _, cmd := range [][]string{
		{"xdg-open", url},
		{"gio", "open", url},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		if err := c.Run(); err == nil {
			return
		}
	}
	log.Printf("could not open browser; open manually: %s", url)
}

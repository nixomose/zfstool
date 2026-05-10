//go:build !cgo || browser_gui

package gui

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// Run starts the same in-process agent + local UI server as the native build, but opens
// the system browser instead of an embedded WebKit window.
func Run(args []string) {
	agentSock, agentURL, noBrowser := parseGUIFlags(args)

	sess, err := StartLocalSession(agentSock, agentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Stop()

	if !noBrowser {
		go openBrowser(sess.BaseURL)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("shutting down (%v)", sig)
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

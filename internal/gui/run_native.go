//go:build cgo && !browser_gui

package gui

import (
	"log"

	webview "github.com/webview/webview_go"
)

// Run opens a native window (WebKit on Linux) with the full web UI, backed by an
// in-process agent and loopback HTTP server unless -agent-url / -agent-socket is set.
func Run(args []string) {
	agentSock, agentURL, _ := parseGUIFlags(args)

	sess, err := StartLocalSession(agentSock, agentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Stop()

	w := webview.New(false)
	defer w.Destroy()

	if err := w.Bind("zfstoolExit", func() { w.Terminate() }); err != nil {
		log.Printf("webview: bind zfstoolExit: %v", err)
	}

	title := "zfstool"
	if sess.Embedded() {
		title += " · local"
	}
	w.SetTitle(title)
	w.SetSize(1100, 720, webview.HintNone)
	w.Navigate(sess.BaseURL)
	w.Run()
}

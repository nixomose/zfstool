//go:build cgo && !browser_gui

package gui

import (
	"log"
	"os"

	webview "github.com/webview/webview_go"
)

// Run opens a native window (WebKit on Linux) with the full web UI, backed by an
// in-process agent and loopback HTTP server unless -agent-url / -agent-socket is set.
func Run(args []string) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		log.Printf("no graphical display detected; using browser mode (run from the desktop for a native window)")
		runBrowserSession(args)
		return
	}

	agentSock, agentURL, _ := parseGUIFlags(args)

	sess, err := StartLocalSession(agentSock, agentURL)
	if err != nil {
		log.Fatal(err)
	}

	initNativeAppIdentity()

	w := webview.New(false)
	if w == nil {
		log.Printf("could not create a native WebKit window; using browser mode")
		sess.Stop()
		runBrowserSession(args)
		return
	}
	defer sess.Stop()
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
	setNativeWindowIcon(w.Window())
	w.Navigate(sess.BaseURL)
	w.Run()
}

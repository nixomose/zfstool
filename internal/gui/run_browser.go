//go:build !cgo || browser_gui

package gui

// Run starts the same in-process agent + local UI server as the native build, but opens
// the system browser instead of an embedded WebKit window.
func Run(args []string) {
	runBrowserSession(args)
}

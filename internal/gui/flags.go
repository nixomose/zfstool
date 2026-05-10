package gui

import "strings"

// parseGUIFlags reads -agent-socket, -agent-url, and -no-browser from args.
// Unknown tokens are ignored so GTK / desktop wrappers can pass extra argv without breaking the GUI.
func parseGUIFlags(args []string) (agentSock, agentURL string, noBrowser bool) {
	if args == nil {
		return "", "", false
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-agent-socket" && i+1 < len(args):
			i++
			agentSock = args[i]
		case strings.HasPrefix(a, "-agent-socket="):
			agentSock = strings.TrimPrefix(a, "-agent-socket=")
		case a == "-agent-url" && i+1 < len(args):
			i++
			agentURL = args[i]
		case strings.HasPrefix(a, "-agent-url="):
			agentURL = strings.TrimPrefix(a, "-agent-url=")
		case a == "-no-browser" || a == "--no-browser":
			noBrowser = true
		}
	}
	return
}

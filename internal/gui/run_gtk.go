//go:build gtk3

package gui

import (
	"context"
	"encoding/json"
	"flag"
	"log"

	"github.com/gotk3/gotk3/gtk"

	"zfstool/internal/client"
)

// Run starts a native GTK3 window with a JSON view of pools (read-only).
func Run(args []string) {
	gtk.Init(nil)

	flg := flag.NewFlagSet("gui", flag.ExitOnError)
	socket := flg.String("agent-socket", defaultSocket(), "zfstool agent unix socket")
	_ = flg.Parse(args)

	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal(err)
	}
	win.SetTitle("zfstool")
	win.SetDefaultSize(960, 720)
	win.Connect("destroy", gtk.MainQuit)

	tv, err := gtk.TextViewNew()
	if err != nil {
		log.Fatal(err)
	}
	tv.SetEditable(false)
	tv.SetMonospace(true)

	buf, err := tv.GetBuffer()
	if err != nil {
		log.Fatal(err)
	}

	c := client.NewUnixHTTPClient(*socket)
	ctx := context.Background()
	type bundle struct {
		Host  interface{} `json:"host"`
		Pools interface{} `json:"pools"`
		Err   string      `json:"error,omitempty"`
	}
	var out bundle
	h, err := c.GetHost(ctx)
	if err != nil {
		out.Err = err.Error()
	} else {
		out.Host = h
	}
	pools, err := c.ListPools(ctx)
	if err != nil {
		if out.Err != "" {
			out.Err += "; "
		}
		out.Err += err.Error()
	} else {
		out.Pools = pools
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	buf.SetText(string(b))

	sw, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	sw.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	sw.Add(tv)

	win.Add(sw)
	win.ShowAll()

	gtk.Main()
}

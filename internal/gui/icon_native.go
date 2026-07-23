//go:build cgo && !browser_gui

package gui

/*
#cgo pkg-config: gtk+-3.0 gdk-pixbuf-2.0
#include <gtk/gtk.h>
#include <gdk-pixbuf/gdk-pixbuf.h>
#include <gio/gio.h>
#include <stdlib.h>
#include <string.h>

static void zfstool_set_prgname(void) {
	g_set_prgname("zfstool");
	gdk_set_program_class("zfstool");
}

// Load PNG bytes into the window icon and the process default icon.
// data must remain valid for the duration of this call (synchronous decode).
static int zfstool_set_window_icon_png(void *window, const void *data, int len, char **err_out) {
	GError *gerr = NULL;
	GInputStream *stream = g_memory_input_stream_new_from_data(data, (gssize)len, NULL);
	if (!stream) {
		if (err_out) *err_out = strdup("g_memory_input_stream_new_from_data failed");
		return 0;
	}
	GdkPixbuf *pixbuf = gdk_pixbuf_new_from_stream(stream, NULL, &gerr);
	g_object_unref(stream);
	if (!pixbuf) {
		if (err_out) {
			if (gerr && gerr->message) *err_out = strdup(gerr->message);
			else *err_out = strdup("gdk_pixbuf_new_from_stream failed");
		}
		if (gerr) g_error_free(gerr);
		return 0;
	}
	gtk_window_set_default_icon(pixbuf);
	if (window) {
		gtk_window_set_icon(GTK_WINDOW(window), pixbuf);
	}
	g_object_unref(pixbuf);
	return 1;
}
*/
import "C"

import (
	_ "embed"
	"log"
	"unsafe"
)

//go:embed icons/zfstool.png
var appIconPNG []byte

func initNativeAppIdentity() {
	C.zfstool_set_prgname()
}

func setNativeWindowIcon(window unsafe.Pointer) {
	if len(appIconPNG) == 0 {
		return
	}
	var errMsg *C.char
	ok := C.zfstool_set_window_icon_png(
		window,
		unsafe.Pointer(&appIconPNG[0]),
		C.int(len(appIconPNG)),
		&errMsg,
	)
	if ok == 0 {
		if errMsg != nil {
			log.Printf("webview: set window icon: %s", C.GoString(errMsg))
			C.free(unsafe.Pointer(errMsg))
		}
	}
}

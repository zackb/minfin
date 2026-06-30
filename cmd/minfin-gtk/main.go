// Command minfin-gtk is a libadwaita (GTK4) desktop app that reads and writes
// the SQLite file directly — no server, no auth. The local file's permissions
// are the trust boundary; the server's users/login exist only to guard an open
// network port, which a desktop app does not have.
//
// Build needs GTK4 + libadwaita dev libraries (pkg-config gtk4 libadwaita-1).
// See `make gtk`.
package main

import (
	"log"
	"os"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"

	"github.com/zackb/minfin/internal/store"
)

func main() {
	st, err := store.Open(getenv("MINFIN_DB", "minfin.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	a := &App{st: st}
	a.app = adw.NewApplication("com.zackb.minfin", gio.ApplicationFlagsNone)
	a.app.ConnectActivate(a.activate)
	if code := a.app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

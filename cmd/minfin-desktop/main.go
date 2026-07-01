// Command minfin-desktop runs the web app as a local single-user desktop app:
// it starts the embedded server on a loopback port and opens the UI in a
// chromeless browser window. Pure Go (no gtk4), so it cross-compiles to a
// dependency-free Windows .exe. See cmd/minfin for the multi-user server.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/env"
	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
	"github.com/zackb/minfin/internal/web"
)

// localEmail identifies the implicit single user of a desktop install.
const localEmail = "local@minfin.local"

func main() {
	st, err := store.Open(env.DBPath())
	if err != nil {
		log.Fatal(err)
	}

	uid, err := ensureLocalUser(st)
	if err != nil {
		log.Fatal(err)
	}

	// auth is bypassed in local mode but NewServer requires a Service; a random
	// dev secret is fine since no tokens are ever issued or checked.
	authSvc, err := auth.New("", true)
	if err != nil {
		log.Fatal(err)
	}
	srv := web.NewServer(st, authSvc)
	srv.SetLocalUser(uid)

	// Loopback-only bind: avoids the Windows Firewall prompt, and :0 lets the OS
	// pick a free port so double-clicks never collide on a fixed one.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String()

	go syncer.SyncAll(st)
	go func() { log.Fatal(http.Serve(ln, srv.Handler())) }()

	// Opening the window blocks until it's closed (when we launched a dedicated
	// browser instance we can wait on); then we exit and the server dies with us.
	openWindow(url)
}

// ensureLocalUser returns the id of the single desktop user, creating it on
// first launch with a throwaway password (never used: auth is bypassed).
func ensureLocalUser(st *store.Store) (string, error) {
	if u, err := st.UserByEmail(localEmail); err == nil {
		return u.ID, nil
	}
	b := make([]byte, 16)
	rand.Read(b)
	hash, err := auth.HashPassword(hex.EncodeToString(b))
	if err != nil {
		return "", err
	}
	u, err := st.CreateUser(localEmail, hash)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

// openWindow shows the app. It prefers a Chromium browser in --app mode (a
// chromeless window) using a throwaway profile, which forces its own process so
// we can block until the user closes the window. If none is found it falls back
// to the default browser and blocks forever (server keeps running).
func openWindow(url string) {
	if bin := findChromium(); bin != "" {
		profile, err := os.MkdirTemp("", "minfin-profile-*")
		if err == nil {
			defer os.RemoveAll(profile)
			cmd := exec.Command(bin, "--app="+url, "--user-data-dir="+profile)
			if err := cmd.Start(); err == nil {
				cmd.Wait()
				return
			}
		}
	}
	openDefault(url)
	select {} // no window to wait on; keep the server alive
}

// findChromium returns the path to an installed Chromium-family browser, or "".
func findChromium() string {
	switch runtime.GOOS {
	case "windows":
		var dirs []string
		for _, e := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if v := os.Getenv(e); v != "" {
				dirs = append(dirs, v)
			}
		}
		rel := []string{
			`Microsoft\Edge\Application\msedge.exe`,
			`Google\Chrome\Application\chrome.exe`,
		}
		for _, d := range dirs {
			for _, r := range rel {
				p := filepath.Join(d, r)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	default:
		for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}
	return ""
}

// openDefault opens url in the user's default browser (a normal tab).
func openDefault(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

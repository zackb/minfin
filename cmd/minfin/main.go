package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
	"github.com/zackb/minfin/internal/web"
)

const legacyTokenFile = "accessurl.txt"

func main() {
	st, err := store.Open(getenv("MINFIN_DB", "minfin.db"))
	if err != nil {
		log.Fatal(err)
	}
	migrateLegacyToken(st)

	if url, _ := st.AccessURL(); url != "" {
		go func() { // non-blocking startup sync
			if err := syncer.Sync(st, url); err != nil {
				log.Printf("startup sync: %v", err)
			}
		}()
	}
	go syncer.Loop(st, 6*time.Hour)

	addr := ":" + getenv("PORT", "8080")
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, web.NewServer(st).Handler()))
}

// migrateLegacyToken imports a pre-existing accessurl.txt into the meta table
// once, so older installs stay connected after the switch to DB-stored tokens.
func migrateLegacyToken(st *store.Store) {
	if u, _ := st.AccessURL(); u != "" {
		return
	}
	b, err := os.ReadFile(legacyTokenFile)
	if err != nil {
		return
	}
	if token := strings.TrimSpace(string(b)); token != "" {
		if err := st.SetAccessURL(token); err != nil {
			log.Printf("migrate token: %v", err)
			return
		}
		log.Println("migrated access token from accessurl.txt")
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
	"github.com/zackb/minfin/internal/web"
)

func main() {
	st, err := store.Open(getenv("MINFIN_DB", "minfin.db"))
	if err != nil {
		log.Fatal(err)
	}

	dev := os.Getenv("MINFIN_DEV") != ""
	authSvc, err := auth.New(os.Getenv("MINFIN_JWT_SECRET"), dev)
	if err != nil {
		log.Fatal(err)
	}

	go syncer.SyncAll(st) // non-blocking startup sync of all portfolios
	go syncer.Loop(st, 6*time.Hour)

	addr := ":" + getenv("PORT", "8080")
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, web.NewServer(st, authSvc).Handler()))
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

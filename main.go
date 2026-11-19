package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed web/dist
var frontend embed.FS

func handler(files fs.FS, app *App) http.Handler {
	mux := http.NewServeMux()
	app.routes(mux)
	mux.HandleFunc("GET /api/targets/local", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "local", "name": "Lokaler Host"})
	})
	mux.HandleFunc("GET /api/", http.NotFound)
	mux.HandleFunc("GET /api", http.NotFound)
	static := http.FileServerFS(files)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Serve the shell on client routes, but preserve 404s for missing assets.
		if strings.HasPrefix(r.URL.Path, "/assets/") || path.Ext(r.URL.Path) != "" {
			static.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, files, "index.html")
	})
	return app.protect(mux)
}

func main() {
	files, err := fs.Sub(frontend, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	app, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: ":8080", Handler: handler(files, app), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

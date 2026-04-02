package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"submanager/internal/api"
	"submanager/internal/fetcher"
	"submanager/internal/service"
	"submanager/internal/store"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(".", "data", "submanager.db")
	}

	dbStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := dbStore.Close(); closeErr != nil {
			log.Printf("failed to close database: %v", closeErr)
		}
	}()

	fetchCacheDir := os.Getenv("FETCH_CACHE_DIR")
	httpFetcher := fetcher.NewHTTPFetcherWithOptions(fetcher.Options{
		CacheDir: fetchCacheDir,
	})
	manager := service.NewManager(dbStore, httpFetcher)
	manager.Start(context.Background())
	superuserToken := os.Getenv("SUPERUSER_TOKEN")
	if superuserToken == "" {
		log.Fatal("SUPERUSER_TOKEN is required")
	}

	handler := api.NewHandler(manager, superuserToken)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Printf("serving without frontend: %v", err)
	}
	var staticFS http.FileSystem
	if distFS != nil {
		staticFS = http.FS(distFS)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler.Routes(staticFS),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("SubManager backend listening on %s with sqlite %s", server.Addr, dbPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

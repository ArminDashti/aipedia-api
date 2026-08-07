package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ArminDashti/aipedia-api/internal/bookmarks"
	"github.com/ArminDashti/aipedia-api/internal/config"
	appdb "github.com/ArminDashti/aipedia-api/internal/db"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	bookmarksDir := os.Getenv("BOOKMARKS_DIR")
	if bookmarksDir == "" {
		bookmarksDir = filepath.Clean(filepath.Join("..", "bookmarks"))
	}

	sqlDB, err := appdb.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	if err := appdb.Migrate(sqlDB, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cats, entries, err := bookmarks.ImportDir(ctx, sqlDB, bookmarksDir)
	if err != nil {
		log.Fatalf("import: %v", err)
	}
	log.Printf("imported %d categories and %d entries from %s", cats, entries, bookmarksDir)
}

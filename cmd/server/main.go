package main

import (
	"log"
	"time"

	"github.com/ArminDashti/aipedia-api/internal/config"
	appdb "github.com/ArminDashti/aipedia-api/internal/db"
	"github.com/ArminDashti/aipedia-api/internal/handlers"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadDotEnv(".env")

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	sqlDB, err := appdb.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	if err := appdb.Migrate(sqlDB, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	h := handlers.New(sqlDB)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", h.Health)
	r.GET("/api/categories", h.ListCategories)
	r.GET("/api/categories/*path", h.CategoryByPath)
	r.GET("/api/entries", h.SearchEntries)

	log.Printf("aipedia-api listening on %s", cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}

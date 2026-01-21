package main

import (
	"goonhub"
	"goonhub/internal/api"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	// Configuration
	dataPath := os.Getenv("DATA_PATH")
	if dataPath == "" {
		dataPath = "./data"
	}

	// Initialize Database
	db, err := data.InitDB("library.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Layers
	videoRepo := data.NewSQLiteVideoRepository(db)
	videoService := core.NewVideoService(videoRepo, dataPath)
	videoHandler := api.NewVideoHandler(videoService)

	r := gin.Default()

	// Register API Routes
	api.RegisterRoutes(r, videoHandler)

	// Health Check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Serve Frontend (Embedded)
	fsys, err := fs.Sub(goonhub.WebDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		f, err := fsys.Open(path)
		if err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			if !stat.IsDir() {
				c.FileFromFS(path, http.FS(fsys))
				return
			}
		}

		data, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "frontend not built"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	log.Println("Starting GoonHub on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

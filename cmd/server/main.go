package main

import (
	"io/fs"
	"log"
	"net/http"

	"goonhub"

	"github.com/gin-gonic/gin"
)

func main() {
	// Set Gin mode
	// gin.SetMode(gin.ReleaseMode) // TODO: Make configurable via flag/env

	r := gin.Default()

	// API Group
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
			})
		})
	}

	// Serve Frontend (Embedded)
	// We need to strip "web/dist" prefix to serve files from root
	fsys, err := fs.Sub(goonhub.WebDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	// Catch-all for SPA and Static files
	// We use NoRoute to avoid conflict with /api group which would happen if we used StaticFS("/")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Strip leading slash
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		// Try to open the file
		f, err := fsys.Open(path)
		if err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			if !stat.IsDir() {
				c.FileFromFS(path, http.FS(fsys))
				return
			}
		}

		// Fallback to index.html for SPA routing
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

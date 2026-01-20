# Project Context: GoonHub

## 1. Project Overview

**GoonHub** is a high-performance, self-hosted web application designed to organize, view, and manage local NSFW video libraries. Ideally, it functions as a "single-binary" application where the frontend is embedded into the backend.

**Core Philosophy:**

- **Privacy First:** No telemetry, local-only storage, panic buttons.
- **Performance:** Capable of handling 10,000+ files with instant search.
- **User Experience:** Netflix-like UI with advanced filtering and VR support.

---

## 2. Technology Stack

### Backend (The Host)

- **Language:** Go (Golang) 1.21+
- **Web Framework:** Gin.
- **Database:** SQLite (embedded) using **GORM** (ORM).
- **Video Processing:** FFmpeg (via `u2takey/ffmpeg-go` wrapper).
- **Image Processing:** `disintegration/imaging` for thumbnails.
- **Filesystem:** `fsnotify` for watching folder changes.

### Frontend (The UI)

- **Framework:** Vue.js 3 with Nuxt 3 (Composition API, `<script setup>`).
- **Build Tool:** Vite (SPA mode).
- **Language:** TypeScript.
- **State Management:** Pinia.
- **Styling:** Tailwind CSS.
- **Video Player:** Plyr.

### Deployment & Distribution

- **Strategy:** "Bake" the compiled frontend `dist` folder into the Go binary using `//go:embed`.
- **Dev Mode:** Frontend runs on port 3000 (proxying API), Backend on port 8080.
- **Prod Mode:** Single binary runs on port 8080.

---

## 3. Architecture & Directory Structure

Adhere strictly to the **Standard Go Project Layout**:

```text
/
├── cmd/
│   └── server/
│       └── main.go       # Entry point. Sets up EmbedFS, DB, and HTTP server.
├── internal/
│   ├── api/              # HTTP Handlers (Gin controllers)
│   ├── core/             # Business Logic (Services)
│   ├── data/             # Database Models & Repositories (GORM)
│   └── jobs/             # Background workers (Scanner, Transcoder)
├── pkg/
│   ├── ffmpeg/           # FFmpeg helper functions
│   └── scraper/          # Logic for ThePornDB/IAFD scraping
├── web/                  # Vue.js Source Code
│   ├── src/
│   └── dist/             # Built assets (Gitignored, but embedded by Go)
├── library.db            # SQLite database file (Gitignored)
└── AGENTS.md              # This file
```
````

---

## 4. Database Schema (Draft)

The AI agent should use these relationships when generating models.

- **Video:** `ID, Path, Title, Hash (MD5/pHash), Duration, Size, Rating, ViewCount, CreatedAt`
    - _HasMany_ Tags, Performers
    - _BelongsTo_ Studio
- **Performer:** `ID, Name, Bio, Birthdate, Gender, ImageURL`
    - _HasMany_ Videos
- **Studio:** `ID, Name, ParentStudioID`
    - _HasMany_ Videos
- **Tag:** `ID, Name, Category` (e.g., Category="Location", Name="Pool")
    - _ManyToMany_ Videos

---

## 5. Key Features & Implementation Details

### A. Library Management

- **File Watcher:** The backend must listen to file system events. When a file is added, trigger a `ScanJob`.
- **Hashing:** Calculate hashes to detect duplicates and for metadata scraping lookups.

### B. Video Player

- **Streaming:** Implement HTTP Range requests in Go to allow seeking.
- **Transcoding:** Check `User-Agent`. If the device doesn't support the codec (e.g., HEVC on old browsers), trigger on-the-fly FFmpeg transcoding.
- **Sprite Sheets:** Generate VTT sprite sheets for timeline hovering.

### C. Privacy & Security

- **Auth:** JWT-based stateless authentication.

### D. Metadata Scraping

- **Scrapers:** Implement modular scrapers (interfaces) for sites like ThePornDB.
- **Logic:** Try to match by Hash first, then by Filename regex.

---

## 6. Coding Rules for the Agent

1.  **Go Error Handling:** NEVER ignore errors. Use `if err != nil`. Wrap errors with context (e.g., `fmt.Errorf("failed to scan file: %w", err)`).
2.  **Concurrency:** Use `sync.WaitGroup` and Channels for the scanner and transcoder. Do not spawn unlimited goroutines; use a Worker Pool pattern.
3.  **Frontend/Backend Contract:**
    - All API responses must be JSON.
    - Use snake_case for JSON keys (`{"video_id": 1}`) and PascalCase for Go Structs (`VideoID int`).
4.  **No Hallucinations:** Do not invent Go standard library functions that don't exist. If using a 3rd party library, verify it is popular and well-maintained.
5.  **Code Blocks:** Always provide the filename at the top of the code block.

---

## 7. Development Workflow (How to run)

To start the project, the agent should assume:

1.  **Backend:** `go run cmd/server/main.go`
2.  **Frontend:** `cd web && bun run dev`

---

*This file is intended to be a living document. As conventions evolve, please update this file.*
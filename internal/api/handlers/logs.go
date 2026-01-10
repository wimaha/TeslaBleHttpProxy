package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/wimaha/TeslaBleHttpProxy/internal/logging"
)

// GetLogs returns log entries as JSON
func GetLogs(w http.ResponseWriter, r *http.Request) {
	storage := logging.GetStorage()

	// Parse query parameters
	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")
	sinceStr := r.URL.Query().Get("since")

	limit := 1000 // default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var since time.Time
	if sinceStr != "" {
		if parsedSince, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsedSince
		}
	}

	entries := storage.GetEntries(level, since, limit)

	// Ensure we always return an array, never null
	if entries == nil {
		entries = []logging.LogEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		http.Error(w, "Failed to encode logs", http.StatusInternalServerError)
		return
	}
}

// GetLogStats returns log statistics as JSON
func GetLogStats(w http.ResponseWriter, r *http.Request) {
	storage := logging.GetStorage()
	stats := storage.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
		return
	}
}

// ShowLogViewer displays the log viewer HTML page
func ShowLogViewer(html fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := LogViewer(w, html); err != nil {
			http.Error(w, "Failed to render log viewer", http.StatusInternalServerError)
		}
	}
}

// LogViewer renders the log viewer template
func LogViewer(w http.ResponseWriter, html fs.FS) error {
	// Use the same parse function pattern as Dashboard
	tmpl := template.Must(
		template.New("html/layout.html").ParseFS(html, "html/layout.html", "html/logs.html"))
	return tmpl.ExecuteTemplate(w, "layout.html", nil)
}

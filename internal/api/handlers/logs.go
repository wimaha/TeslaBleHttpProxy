package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"strconv"
	"text/template"
	"time"

	"github.com/wimaha/TeslaBleHttpProxy/internal/api/models"
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

// RebootSystem initiates a Raspberry Pi reboot.
// The reboot is executed asynchronously so the API can return first.
func RebootSystem(w http.ResponseWriter, r *http.Request) {
	var response models.Response
	response.Command = "system_reboot"
	response.Result = false

	defer commonDefer(w, &response)

	logging.Warn("System reboot requested", "remote_addr", r.RemoteAddr)

	go func() {
		// Give the HTTP response time to flush before rebooting the host.
		time.Sleep(750 * time.Millisecond)
		if err := executeReboot(); err != nil {
			logging.Error("Failed to execute reboot command", "error", err)
		}
	}()

	response.Result = true
	response.Reason = "System reboot initiated. The device will restart shortly."
}

func executeReboot() error {
	commands := [][]string{
		{"sudo", "shutdown", "-r", "now"},
		{"/sbin/shutdown", "-r", "now"},
		{"shutdown", "-r", "now"},
	}

	var lastErr error
	for _, args := range commands {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		err := cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("all reboot commands failed: %w", lastErr)
}

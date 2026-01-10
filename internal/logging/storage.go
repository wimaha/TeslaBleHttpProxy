package logging

import (
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

const (
	// MaxLogEntries is the maximum number of log entries to keep in memory
	MaxLogEntries = 10000
	// LogRetentionHours is how long to keep logs (default: 7 days)
	LogRetentionHours = 24 * 7
	// CleanupInterval is how often to run cleanup (default: 1 hour)
	CleanupInterval = time.Hour
)

type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type LogStorage struct {
	entries []LogEntry
	mu      sync.RWMutex
	writer  *log.Logger
}

var storage *LogStorage
var storageOnce sync.Once

// GetStorage returns the singleton log storage instance
func GetStorage() *LogStorage {
	storageOnce.Do(func() {
		storage = &LogStorage{
			entries: make([]LogEntry, 0, MaxLogEntries),
			writer:  log.New(os.Stderr),
		}
		// Start cleanup routine
		go storage.cleanupRoutine()
	})
	return storage
}

// AddEntry adds a log entry to storage
func (ls *LogStorage) AddEntry(level string, message string, fields map[string]interface{}) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	// If we're at capacity, remove the oldest entry
	if len(ls.entries) >= MaxLogEntries {
		ls.entries = ls.entries[1:]
	}

	ls.entries = append(ls.entries, entry)
}

// getLevelPriority returns a numeric priority for log levels (higher = more severe)
// Used for hierarchical filtering where selecting a level shows that level and all higher priority levels
func getLevelPriority(level string) int {
	switch level {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	case "fatal":
		return 4
	default:
		return -1 // Unknown level
	}
}

// shouldIncludeLevel checks if an entry level should be included based on minimum level filter
// Returns true if entryLevel should be shown when filtering by minLevel
func shouldIncludeLevel(entryLevel, minLevel string) bool {
	if minLevel == "" {
		return true // No filter, show all
	}

	entryPriority := getLevelPriority(entryLevel)
	minPriority := getLevelPriority(minLevel)

	// Invalid levels are not included
	if entryPriority < 0 || minPriority < 0 {
		return false
	}

	// Include entry if its priority is >= minimum level priority
	// This means: debug shows all, info shows info+, warn shows warn+, etc.
	return entryPriority >= minPriority
}

// GetEntries returns log entries, optionally filtered by level (hierarchical) and time range
func (ls *LogStorage) GetEntries(level string, since time.Time, limit int) []LogEntry {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	var filtered []LogEntry
	count := 0

	// Iterate backwards to get most recent entries first
	for i := len(ls.entries) - 1; i >= 0 && count < limit; i-- {
		entry := ls.entries[i]

		// Filter by level hierarchically (if specified)
		// Example: "info" shows info, warn, error, fatal (but not debug)
		if !shouldIncludeLevel(entry.Level, level) {
			continue
		}

		// Filter by time if specified
		if !since.IsZero() && entry.Timestamp.Before(since) {
			continue
		}

		filtered = append(filtered, entry)
		count++
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// GetRecentEntries returns the most recent N entries
func (ls *LogStorage) GetRecentEntries(limit int) []LogEntry {
	return ls.GetEntries("", time.Time{}, limit)
}

// GetStats returns statistics about stored logs
func (ls *LogStorage) GetStats() map[string]interface{} {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	stats := map[string]interface{}{
		"total_entries":   len(ls.entries),
		"max_entries":     MaxLogEntries,
		"retention_hours": LogRetentionHours,
	}

	// Count by level
	levelCounts := make(map[string]int)
	for _, entry := range ls.entries {
		levelCounts[entry.Level]++
	}
	stats["level_counts"] = levelCounts

	// Oldest and newest timestamps
	if len(ls.entries) > 0 {
		stats["oldest_entry"] = ls.entries[0].Timestamp
		stats["newest_entry"] = ls.entries[len(ls.entries)-1].Timestamp
	}

	return stats
}

// cleanupRoutine periodically removes old log entries
func (ls *LogStorage) cleanupRoutine() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ls.cleanup()
	}
}

// cleanup removes log entries older than the retention period
func (ls *LogStorage) cleanup() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(LogRetentionHours) * time.Hour)
	originalLen := len(ls.entries)

	// Remove entries older than cutoff
	keepIndex := 0
	for i, entry := range ls.entries {
		if entry.Timestamp.After(cutoff) {
			ls.entries[keepIndex] = ls.entries[i]
			keepIndex++
		}
	}

	ls.entries = ls.entries[:keepIndex]

	if removed := originalLen - len(ls.entries); removed > 0 {
		// Note: We don't log cleanup events to avoid recursion
		// The cleanup happens silently in the background
		_ = removed // Acknowledge that logs were removed
	}
}

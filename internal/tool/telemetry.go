package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TelemetryEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Tool       string                 `json:"tool"`
	Params     map[string]interface{} `json:"params"`
	Success    bool                   `json:"success"`
	DurationMs int64                  `json:"duration_ms"`
	SnapshotID string                 `json:"snapshot_id,omitempty"`
	RolledBack bool                   `json:"rolled_back,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type TelemetryLogger struct {
	mu      sync.Mutex
	logFile string
	recent  []TelemetryEntry // in-memory ring for display
	maxKeep int
}

func NewTelemetryLogger(dataDir string) *TelemetryLogger {
	return &TelemetryLogger{
		logFile: filepath.Join(dataDir, "telemetry.jsonl"),
		maxKeep: 100,
	}
}

func (t *TelemetryLogger) Log(entry TelemetryEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recent = append(t.recent, entry)
	if len(t.recent) > t.maxKeep {
		t.recent = t.recent[1:]
	}

	// Print to console
	status := "\033[32m✓\033[0m"
	extra := ""
	if !entry.Success {
		status = "\033[31m✗\033[0m"
		if entry.RolledBack {
			extra = " (rollback)"
		}
	}
	snapInfo := ""
	if entry.SnapshotID != "" {
		snapInfo = " " + entry.SnapshotID
	}
	fmt.Printf("\033[33m[Telemetry] %s %s %s %.1fs%s%s\033[0m\n",
		entry.Tool, formatParams(entry.Params), status,
		float64(entry.DurationMs)/1000.0, snapInfo, extra)

	// Append to file
	if err := os.MkdirAll(filepath.Dir(t.logFile), 0755); err != nil {
		return
	}
	f, err := os.OpenFile(t.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(entry)
	f.Write(data)
	f.WriteString("\n")
}

func (t *TelemetryLogger) Recent(n int) []TelemetryEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n > len(t.recent) {
		n = len(t.recent)
	}
	out := make([]TelemetryEntry, n)
	copy(out, t.recent[len(t.recent)-n:])
	return out
}

func (t *TelemetryLogger) FormatRecent(n int) string {
	entries := t.Recent(n)
	if len(entries) == 0 {
		return "没有操作记录。\n"
	}
	out := ""
	for _, e := range entries {
		ts := e.Timestamp.Format("15:04:05")
		status := "\033[32m✓\033[0m"
		extra := ""
		if !e.Success {
			status = "\033[31m✗\033[0m"
			if e.RolledBack {
				extra = " (rollback)"
			}
		}
		snapInfo := ""
		if e.SnapshotID != "" {
			snapInfo = " " + e.SnapshotID
		}
		out += fmt.Sprintf("  [%s] %-16s %-40s %s %.1fs%s%s\n",
			ts, e.Tool, formatParams(e.Params), status,
			float64(e.DurationMs)/1000.0, snapInfo, extra)
	}
	return out
}

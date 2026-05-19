package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// handleStreamLogs streams build output for a release using Server-Sent Events.
// The client receives the current log snapshot and then polls until the build
// finishes, after which a final "done" event is sent.
func (s *APIServer) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	releaseID := vars["release_id"]

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastLen int
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			status, output, err := s.buildManager.GetBuildLogs(releaseID)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: release not found\n\n")
				flusher.Flush()
				return
			}

			// Only send new output since last tick
			if len(output) > lastLen {
				newData := output[lastLen:]
				lastLen = len(output)
				fmt.Fprintf(w, "data: %s\n\n", sseEscape(newData))
				flusher.Flush()
			}

			if status == "succeeded" || status == "failed" {
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", status)
				flusher.Flush()
				return
			}
		}
	}
}

// sseEscape ensures multi-line log output is valid SSE by prefixing
// each line with "data: ".
func sseEscape(s string) string {
	// For simplicity, send as a single data field with \n replaced by
	// the SSE continuation format.
	result := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			result += "\ndata: "
		}
		result += line
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	if len(lines) == 0 {
		return []string{s}
	}
	return lines
}

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type contextKey string

const contextKeyTokenID contextKey = "token_id"

type AuditEvent struct {
	Timestamp  string `json:"ts"`
	TokenID    string `json:"token_id,omitempty"`
	Action     string `json:"action"`
	Resource   string `json:"resource,omitempty"`
	RemoteIP   string `json:"remote_ip"`
	StatusCode int    `json:"status,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

func auditLog(r *http.Request, action, resource, detail string) {
	tokenID := ""
	if v := r.Context().Value(contextKeyTokenID); v != nil {
		tokenID = v.(string)
	}
	ev := AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TokenID:   tokenID,
		Action:    action,
		Resource:  resource,
		RemoteIP:  realIP(r),
		Detail:    detail,
	}
	b, _ := json.Marshal(ev)
	log.Printf("[AUDIT] %s", b)
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

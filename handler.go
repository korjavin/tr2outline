package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// WebhookServer handles incoming webhooks and manages Outline integration.
type WebhookServer struct {
	cfg           *Config
	outlineClient *OutlineClient
}

// NewWebhookServer creates a new WebhookServer instance.
func NewWebhookServer(cfg *Config, outlineClient *OutlineClient) *WebhookServer {
	return &WebhookServer{
		cfg:           cfg,
		outlineClient: outlineClient,
	}
}

// RegisterRoutes registers HTTP routes on the provided mux.
func (s *WebhookServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/webhooks/anarlog", s.HandleAnarlogWebhook)
	mux.HandleFunc("GET /health", s.HandleHealth)
	mux.HandleFunc("GET /healthz", s.HandleHealth)
}

// HandleHealth returns 200 OK for healthchecks.
func (s *WebhookServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HandleAnarlogWebhook processes incoming webhooks from Anarlog.
func (s *WebhookServer) HandleAnarlogWebhook(w http.ResponseWriter, r *http.Request) {
	// Read raw body (limited to 10MB to prevent memory exhaustion)
	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 1. Verify HMAC-SHA256 signature
	signatureHeader := r.Header.Get("x-anarlog-signature")
	if !VerifyAnySignature(rawBody, signatureHeader, s.cfg.AnarlogWebhookSecrets) {
		log.Printf("Unauthorized: signature verification failed for remote IP %s", r.RemoteAddr)
		http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
		return
	}

	// 2. Parse payload JSON
	payload, err := ParseAnarlogPayload(rawBody, r.Header.Get("x-anarlog-timestamp"))
	if err != nil {
		bodySnippet := string(rawBody)
		if len(bodySnippet) > 1000 {
			bodySnippet = bodySnippet[:1000] + "... (truncated)"
		}
		log.Printf("Bad Request: failed to parse JSON payload: %v | Body: %s", err, bodySnippet)
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// Determine event type from header or payload
	eventType := r.Header.Get("x-anarlog-event")
	if eventType == "" {
		eventType = payload.Event
	}

	// 3. Filter events: process "note.enhanced", update events, and "meeting.completed" if it has content
	shouldProcess := false
	skipReason := ""

	hasMeetingData := strings.TrimSpace(payload.Data.Meeting.Title) != "" ||
		len(payload.Data.Meeting.Summaries) > 0 ||
		strings.TrimSpace(payload.Data.TranscriptText) != ""

	switch eventType {
	case "note.enhanced", "meeting.updated", "note.updated", "note.created":
		shouldProcess = true
	case "meeting.completed":
		// When a meeting first finishes recording, it has no title or summaries yet.
		// Skip initial empty meeting.completed to wait for AI enhancement.
		// If it already has title or summaries (e.g. edited existing meeting), process and update Outline.
		if hasMeetingData && (payload.Data.Meeting.Title != "" || len(payload.Data.Meeting.Summaries) > 0) {
			shouldProcess = true
		} else {
			skipReason = "meeting.completed has no title or summaries yet (waiting for AI enhancement)"
		}
	default:
		skipReason = fmt.Sprintf("event %q is not a target meeting event", eventType)
	}

	if !shouldProcess {
		log.Printf("Info: received non-target event %q (id: %s). Skipping without Outline call (%s).", eventType, payload.ID, skipReason)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ignored",
			"event":   eventType,
			"message": fmt.Sprintf("Event received but ignored: %s", skipReason),
		})
		return
	}

	// 4. Format Markdown and Title
	title := FormatDocumentTitle(payload.Data.Meeting.Title, payload.CreatedAt)
	markdownText := FormatMeetingMarkdown(payload)

	log.Printf("Processing meeting note: %q (event ID: %s, meeting ID: %s)", title, payload.ID, payload.Data.Meeting.ID)

	// 5. Send to Outline API (update if document exists, otherwise create)
	docResp, updated, err := s.outlineClient.CreateOrUpdateDocument(r.Context(), payload.Data.Meeting.ID, title, markdownText)
	if err != nil {
		log.Printf("Error: failed to save Outline document for event %s: %v", payload.ID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to save document in Outline",
		})
		return
	}

	action := "created"
	if updated {
		action = "updated"
	}
	log.Printf("Successfully %s Outline document %q (ID: %s, URL: %s)", action, docResp.Data.Title, docResp.Data.ID, docResp.Data.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"action":      action,
		"document_id": docResp.Data.ID,
		"title":       docResp.Data.Title,
		"url":         docResp.Data.URL,
	})
}

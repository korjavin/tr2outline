package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
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
	var payload AnarlogWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		log.Printf("Bad Request: failed to parse JSON payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Determine event type from header or payload
	eventType := r.Header.Get("x-anarlog-event")
	if eventType == "" {
		eventType = payload.Event
	}

	// 3. Filter events: only process "note.enhanced"
	if eventType != "note.enhanced" {
		log.Printf("Info: received non-target event %q (id: %s). Skipping without Outline call.", eventType, payload.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ignored",
			"event":   eventType,
			"message": "Event received but ignored (only 'note.enhanced' is processed)",
		})
		return
	}

	// 4. Format Markdown and Title
	title := FormatDocumentTitle(payload.Data.Meeting.Title, payload.CreatedAt)
	markdownText := FormatMeetingMarkdown(&payload)

	log.Printf("Processing meeting note: %q (event ID: %s, meeting ID: %s)", title, payload.ID, payload.Data.Meeting.ID)

	// 5. Send to Outline API
	docResp, err := s.outlineClient.CreateDocument(r.Context(), title, markdownText)
	if err != nil {
		log.Printf("Error: failed to create Outline document for event %s: %v", payload.ID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to create document in Outline",
		})
		return
	}

	log.Printf("Successfully created Outline document %q (ID: %s, URL: %s)", docResp.Data.Title, docResp.Data.ID, docResp.Data.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"document_id": docResp.Data.ID,
		"title":       docResp.Data.Title,
		"url":         docResp.Data.URL,
	})
}

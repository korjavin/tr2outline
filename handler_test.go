package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerHealth(t *testing.T) {
	server := NewWebhookServer(&Config{}, nil)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestHandler_InvalidSignature(t *testing.T) {
	cfg := &Config{
		AnarlogWebhookSecrets: []string{"test-secret"},
	}
	server := NewWebhookServer(cfg, nil)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	payload := []byte(`{"event":"note.enhanced"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/anarlog", bytes.NewReader(payload))
	req.Header.Set("x-anarlog-signature", "sha256=invalid")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestHandler_NonTargetEvent(t *testing.T) {
	cfg := &Config{
		AnarlogWebhookSecrets: []string{"test-secret"},
	}
	server := NewWebhookServer(cfg, nil)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	payload := []byte(`{"event":"webhook.test","data":{}}`)
	sig := ComputeSignature(payload, "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/anarlog", bytes.NewReader(payload))
	req.Header.Set("x-anarlog-signature", sig)
	req.Header.Set("x-anarlog-event", "webhook.test")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp["status"] != "ignored" {
		t.Fatalf("expected status 'ignored', got %v", resp["status"])
	}
}

func TestHandler_Success(t *testing.T) {
	// Mock Outline server
	var capturedOutlineReq OutlineDocumentCreateRequest
	var capturedAuthHeader string

	mockOutline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents.create" {
			http.NotFound(w, r)
			return
		}
		capturedAuthHeader = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedOutlineReq)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"data": {
				"id": "doc_12345",
				"title": "` + capturedOutlineReq.Title + `",
				"url": "https://app.getoutline.com/doc/doc_12345"
			}
		}`))
	}))
	defer mockOutline.Close()

	cfg := &Config{
		AnarlogWebhookSecrets: []string{"device1-secret", "device2-secret"},
		OutlineURL:            mockOutline.URL,
		OutlineAPIKey:         "outline-token-xyz",
		OutlineCollectionID:   "col_target_abc",
	}
	outlineClient := NewOutlineClient(cfg.OutlineURL, cfg.OutlineAPIKey, cfg.OutlineCollectionID)
	server := NewWebhookServer(cfg, outlineClient)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	rawJSON := `{
		"id": "evt_123",
		"event": "note.enhanced",
		"created_at": "2026-07-28T09:00:00.000Z",
		"data": {
			"meeting": {
				"id": "meet_abc",
				"title": "Weekly Sync",
				"note": "Raw notes...",
				"summaries": ["Summary point 1", "Summary point 2"],
				"participants": ["Alice", "Bob"],
				"action_items": ["Alice to update DB", "Bob to call client"]
			},
			"transcript_text": "Full transcript goes here..."
		}
	}`

	// Sign using the SECOND device's secret
	sig := ComputeSignature([]byte(rawJSON), "device2-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/anarlog", strings.NewReader(rawJSON))
	req.Header.Set("x-anarlog-signature", sig)
	req.Header.Set("x-anarlog-event", "note.enhanced")
	req.Header.Set("x-anarlog-timestamp", "1785229200")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify headers and body sent to Outline
	if capturedAuthHeader != "Bearer outline-token-xyz" {
		t.Errorf("expected Authorization Bearer outline-token-xyz, got %q", capturedAuthHeader)
	}
	if capturedOutlineReq.CollectionID != "col_target_abc" {
		t.Errorf("expected collection col_target_abc, got %q", capturedOutlineReq.CollectionID)
	}
	if capturedOutlineReq.Title != "Meeting: Weekly Sync (2026-07-28)" {
		t.Errorf("expected title 'Meeting: Weekly Sync (2026-07-28)', got %q", capturedOutlineReq.Title)
	}
	if !capturedOutlineReq.Publish {
		t.Errorf("expected publish to be true")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}
	if resp["document_id"] != "doc_12345" {
		t.Errorf("expected document_id 'doc_12345', got %v", resp["document_id"])
	}
}

func TestHandler_OutlineError(t *testing.T) {
	// Mock Outline server returning 500
	mockOutline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"ok":false,"message":"Collection not found"}`, http.StatusBadRequest)
	}))
	defer mockOutline.Close()

	cfg := &Config{
		AnarlogWebhookSecrets: []string{"test-secret"},
		OutlineURL:            mockOutline.URL,
		OutlineAPIKey:         "outline-token-xyz",
		OutlineCollectionID:   "col_target_abc",
	}
	outlineClient := NewOutlineClient(cfg.OutlineURL, cfg.OutlineAPIKey, cfg.OutlineCollectionID)
	server := NewWebhookServer(cfg, outlineClient)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	rawJSON := `{"event":"note.enhanced","data":{"meeting":{"title":"Test"}}}`
	sig := ComputeSignature([]byte(rawJSON), "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/anarlog", strings.NewReader(rawJSON))
	req.Header.Set("x-anarlog-signature", sig)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestHandler_UpdateExisting(t *testing.T) {
	var updateCalled bool
	var updateDocID string

	mockOutline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/documents.search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": [
					{
						"document": {
							"id": "existing-doc-999",
							"title": "Meeting: Update Test (2026-09-06)",
							"collectionId": "col_target_abc"
						}
					}
				]
			}`))
		case "/api/documents.update":
			updateCalled = true
			body, _ := io.ReadAll(r.Body)
			var updateReq OutlineDocumentUpdateRequest
			_ = json.Unmarshal(body, &updateReq)
			updateDocID = updateReq.ID

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"id": "existing-doc-999",
					"title": "` + updateReq.Title + `",
					"url": "https://app.getoutline.com/doc/existing-doc-999"
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockOutline.Close()

	cfg := &Config{
		AnarlogWebhookSecrets: []string{"test-secret"},
		OutlineURL:            mockOutline.URL,
		OutlineAPIKey:         "outline-token-xyz",
		OutlineCollectionID:   "col_target_abc",
	}
	outlineClient := NewOutlineClient(cfg.OutlineURL, cfg.OutlineAPIKey, cfg.OutlineCollectionID)
	server := NewWebhookServer(cfg, outlineClient)

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	rawJSON := `{"event":"note.enhanced","created_at":"2026-09-06T12:00:00Z","data":{"meeting":{"id":"meet-111","title":"Update Test"}}}`
	sig := ComputeSignature([]byte(rawJSON), "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/anarlog", strings.NewReader(rawJSON))
	req.Header.Set("x-anarlog-signature", sig)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !updateCalled {
		t.Errorf("expected documents.update to be called for existing document")
	}
	if updateDocID != "existing-doc-999" {
		t.Errorf("expected updated doc ID 'existing-doc-999', got %q", updateDocID)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["action"] != "updated" {
		t.Errorf("expected action 'updated', got %v", resp["action"])
	}
}

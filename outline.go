package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OutlineClient handles communication with the Outline API.
type OutlineClient struct {
	BaseURL      string
	APIKey       string
	CollectionID string
	HTTPClient   *http.Client
}

// NewOutlineClient creates a new OutlineClient instance.
func NewOutlineClient(baseURL, apiKey, collectionID string) *OutlineClient {
	return &OutlineClient{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		CollectionID: collectionID,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// CreateDocument creates a new document in Outline.
func (c *OutlineClient) CreateDocument(ctx context.Context, title, markdownText string) (*OutlineDocumentCreateResponse, error) {
	reqBody := OutlineDocumentCreateRequest{
		CollectionID: c.CollectionID,
		Title:        title,
		Text:         markdownText,
		Publish:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal outline request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/documents.create", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to build outline http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to outline API failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read outline response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("outline API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var outlineResp OutlineDocumentCreateResponse
	if err := json.Unmarshal(respBody, &outlineResp); err != nil {
		return nil, fmt.Errorf("failed to decode outline response: %w", err)
	}

	return &outlineResp, nil
}

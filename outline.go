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

// SearchDocuments searches for documents in Outline matching query.
func (c *OutlineClient) SearchDocuments(ctx context.Context, query string) ([]OutlineDocumentSearchResult, error) {
	reqBody := OutlineDocumentSearchRequest{
		Query:        query,
		CollectionID: c.CollectionID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/documents.search", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to build search http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp OutlineDocumentSearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return searchResp.Data, nil
}

// UpdateDocument updates an existing document in Outline.
func (c *OutlineClient) UpdateDocument(ctx context.Context, docID, title, markdownText string) (*OutlineDocumentCreateResponse, error) {
	reqBody := OutlineDocumentUpdateRequest{
		ID:      docID,
		Title:   title,
		Text:    markdownText,
		Publish: true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/documents.update", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to build update http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read update response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var updateResp OutlineDocumentCreateResponse
	if err := json.Unmarshal(respBody, &updateResp); err != nil {
		return nil, fmt.Errorf("failed to decode update response: %w", err)
	}

	return &updateResp, nil
}

// CreateOrUpdateDocument searches for an existing document for this meeting, updating it if found, or creating a new one.
func (c *OutlineClient) CreateOrUpdateDocument(ctx context.Context, meetingID, title, markdownText string) (*OutlineDocumentCreateResponse, bool, error) {
	var targetDocID string

	// 1. Try search by meeting ID if provided
	if meetingID != "" {
		if results, err := c.SearchDocuments(ctx, meetingID); err == nil {
			for _, r := range results {
				if c.CollectionID == "" || r.Document.CollectionID == c.CollectionID {
					targetDocID = r.Document.ID
					break
				}
			}
		}
	}

	// 2. If not found by meeting ID, try searching by exact title
	if targetDocID == "" && title != "" {
		if results, err := c.SearchDocuments(ctx, title); err == nil {
			for _, r := range results {
				if r.Document.Title == title && (c.CollectionID == "" || r.Document.CollectionID == c.CollectionID) {
					targetDocID = r.Document.ID
					break
				}
			}
		}
	}

	// 3. Update existing document if found
	if targetDocID != "" {
		resp, err := c.UpdateDocument(ctx, targetDocID, title, markdownText)
		if err == nil {
			return resp, true, nil
		}
	}

	// 4. Fallback to Create
	resp, err := c.CreateDocument(ctx, title, markdownText)
	return resp, false, err
}

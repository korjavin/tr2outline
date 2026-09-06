package main

// AnarlogWebhookPayload represents the incoming webhook JSON body from Anarlog.
type AnarlogWebhookPayload struct {
	ID        string             `json:"id"`
	Event     string             `json:"event"`
	CreatedAt string             `json:"created_at"`
	Data      AnarlogPayloadData `json:"data"`
}

// AnarlogPayloadData contains meeting details and full transcript.
type AnarlogPayloadData struct {
	Meeting        MeetingData `json:"meeting"`
	TranscriptText string      `json:"transcript_text"`
}

// MeetingData contains detailed information about the meeting.
type MeetingData struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Note         string   `json:"note"`
	Summaries    []string `json:"summaries"`
	Participants []string `json:"participants"`
	ActionItems  []string `json:"action_items"`
}

// OutlineDocumentCreateRequest represents the JSON body sent to Outline /api/documents.create.
type OutlineDocumentCreateRequest struct {
	CollectionID string `json:"collectionId"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	Publish      bool   `json:"publish"`
}

// OutlineDocumentCreateResponse represents the response from Outline /api/documents.create or documents.update.
type OutlineDocumentCreateResponse struct {
	Data struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"data"`
	Ok      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// OutlineDocumentSearchRequest represents the JSON body sent to Outline /api/documents.search.
type OutlineDocumentSearchRequest struct {
	Query        string `json:"query"`
	CollectionID string `json:"collectionId,omitempty"`
}

// OutlineDocumentSearchResult represents an item in Outline /api/documents.search results.
type OutlineDocumentSearchResult struct {
	Document struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		URL          string `json:"url"`
		CollectionID string `json:"collectionId"`
	} `json:"document"`
}

// OutlineDocumentSearchResponse represents the response from Outline /api/documents.search.
type OutlineDocumentSearchResponse struct {
	Data []OutlineDocumentSearchResult `json:"data"`
	Ok   bool                          `json:"ok"`
}

// OutlineDocumentUpdateRequest represents the JSON body sent to Outline /api/documents.update.
type OutlineDocumentUpdateRequest struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text,omitempty"`
	Publish bool   `json:"publish"`
}

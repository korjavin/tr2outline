package main

import (
	"strings"
	"testing"
)

func TestFormatMeetingMarkdown(t *testing.T) {
	payload := &AnarlogWebhookPayload{
		ID:        "evt_123",
		Event:     "note.enhanced",
		CreatedAt: "2026-07-28T09:00:00.000Z",
		Data: AnarlogPayloadData{
			Meeting: MeetingData{
				ID:    "meet_abc",
				Title: "Weekly Sync",
				Note:  "Raw notes...",
				Summaries: []string{
					"Summary point 1",
					"Summary point 2",
				},
				Participants: []string{
					"Alice",
					"Bob",
				},
				ActionItems: []string{
					"Alice to update DB",
					"Bob to call client",
				},
			},
			TranscriptText: "Full transcript goes here...",
		},
	}

	title := FormatDocumentTitle(payload.Data.Meeting.Title, payload.CreatedAt)
	expectedTitle := "Meeting: Weekly Sync (2026-07-28)"
	if title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, title)
	}

	md := FormatMeetingMarkdown(payload)

	expectedSnippets := []string{
		"# Meeting: Weekly Sync",
		"**Date:** 2026-07-28 09:00:00 UTC",
		"**Participants:** Alice, Bob",
		"## 📝 Summary",
		"- Summary point 1",
		"- Summary point 2",
		"## ✅ Action Items",
		"- [ ] Alice to update DB",
		"- [ ] Bob to call client",
		"## 🗒️ Notes",
		"Raw notes...",
		"<details>",
		"<summary><b>🎙️ Full Transcript</b></summary>",
		"Full transcript goes here...",
		"</details>",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(md, snippet) {
			t.Errorf("expected markdown to contain %q, but got:\n%s", snippet, md)
		}
	}
}

func TestFormatMeetingMarkdown_EmptyFields(t *testing.T) {
	payload := &AnarlogWebhookPayload{
		ID:        "evt_empty",
		Event:     "note.enhanced",
		CreatedAt: "invalid-date",
		Data: AnarlogPayloadData{
			Meeting: MeetingData{
				Title: "",
				Note:  "",
			},
			TranscriptText: "",
		},
	}

	title := FormatDocumentTitle(payload.Data.Meeting.Title, payload.CreatedAt)
	if !strings.HasPrefix(title, "Meeting: Untitled") {
		t.Errorf("expected title to start with 'Meeting: Untitled', got %q", title)
	}

	md := FormatMeetingMarkdown(payload)

	expectedSnippets := []string{
		"# Meeting: Untitled",
		"**Participants:** _None_",
		"_No summary provided._",
		"_No action items._",
		"_No notes._",
		"_No transcript available._",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(md, snippet) {
			t.Errorf("expected markdown to contain %q, but got:\n%s", snippet, md)
		}
	}
}

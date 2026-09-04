package main

import (
	"testing"
)

func TestParseAnarlogPayload_StrictFormat(t *testing.T) {
	raw := []byte(`{
		"id": "evt_123",
		"event": "note.enhanced",
		"created_at": "2026-07-28T09:00:00.000Z",
		"data": {
			"meeting": {
				"id": "meet_abc",
				"title": "Weekly Sync",
				"note": "Raw notes...",
				"summaries": ["Point 1", "Point 2"],
				"participants": ["Alice", "Bob"],
				"action_items": ["Alice to update DB"]
			},
			"transcript_text": "Full transcript"
		}
	}`)

	payload, err := ParseAnarlogPayload(raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.ID != "evt_123" {
		t.Errorf("expected evt_123, got %s", payload.ID)
	}
	if payload.Event != "note.enhanced" {
		t.Errorf("expected note.enhanced, got %s", payload.Event)
	}
	if payload.Data.Meeting.Title != "Weekly Sync" {
		t.Errorf("expected Weekly Sync, got %s", payload.Data.Meeting.Title)
	}
	if len(payload.Data.Meeting.Summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(payload.Data.Meeting.Summaries))
	}
	if len(payload.Data.Meeting.ActionItems) != 1 {
		t.Errorf("expected 1 action item, got %d", len(payload.Data.Meeting.ActionItems))
	}
}

func TestParseAnarlogPayload_FlexibleTypes(t *testing.T) {
	// Numeric timestamp, action items as objects, summaries as string, participants as objects
	raw := []byte(`{
		"id": "evt_456",
		"type": "note.enhanced",
		"created_at": 1785229200,
		"data": {
			"meeting": {
				"title": "Strategy Meeting",
				"note": "Discussion",
				"summaries": "Point A\nPoint B",
				"participants": [
					{"name": "Alice Smith"},
					{"displayName": "Bob Jones"}
				],
				"action_items": [
					{"task": "Prepare report"},
					{"content": "Call supplier"}
				]
			},
			"transcript": [
				{"speaker": "Alice", "text": "Hello"},
				{"speaker": "Bob", "text": "Hi there"}
			]
		}
	}`)

	payload, err := ParseAnarlogPayload(raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.Event != "note.enhanced" {
		t.Errorf("expected note.enhanced, got %s", payload.Event)
	}
	if payload.Data.Meeting.Title != "Strategy Meeting" {
		t.Errorf("expected Strategy Meeting, got %s", payload.Data.Meeting.Title)
	}
	if len(payload.Data.Meeting.Summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(payload.Data.Meeting.Summaries))
	}
	if len(payload.Data.Meeting.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(payload.Data.Meeting.Participants))
	}
	if payload.Data.Meeting.Participants[0] != "Alice Smith" {
		t.Errorf("expected Alice Smith, got %s", payload.Data.Meeting.Participants[0])
	}
	if len(payload.Data.Meeting.ActionItems) != 2 {
		t.Errorf("expected 2 action items, got %d", len(payload.Data.Meeting.ActionItems))
	}
	if payload.Data.Meeting.ActionItems[0] != "Prepare report" {
		t.Errorf("expected Prepare report, got %s", payload.Data.Meeting.ActionItems[0])
	}
	if payload.Data.TranscriptText == "" {
		t.Errorf("expected transcript text to be populated from segments")
	}
}

func TestParseAnarlogPayload_NoteAsObject(t *testing.T) {
	// The exact issue reported: note is an object {"content": "...", "sections": ...}
	raw := []byte(`{
		"id": "evt_789",
		"event": "note.enhanced",
		"created_at": "2026-09-04T23:09:00Z",
		"data": {
			"meeting": {
				"title": "Call with Client",
				"note": {
					"content": "Meeting went well. Agreement reached on deliverables.",
					"summary": "Short summary"
				},
				"summaries": ["Good discussion"],
				"participants": ["Ivan", "John"],
				"action_items": ["Send invoice"]
			},
			"transcript_text": "Call audio transcript..."
		}
	}`)

	payload, err := ParseAnarlogPayload(raw, "")
	if err != nil {
		t.Fatalf("unexpected error parsing payload with note as object: %v", err)
	}

	if payload.Data.Meeting.Note != "Meeting went well. Agreement reached on deliverables." {
		t.Errorf("expected note content, got: %q", payload.Data.Meeting.Note)
	}
}


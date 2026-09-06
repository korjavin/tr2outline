package main

import (
	"strings"
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

func TestParseAnarlogPayload_NestedNoteFields(t *testing.T) {
	// Payload where title and summary are inside note or at data level instead of meeting directly
	raw := []byte(`{
		"id": "evt_nested",
		"event": "note.enhanced",
		"data": {
			"meeting": {
				"id": "meet_xyz",
				"note": {
					"title": "Nested Architecture Review",
					"summary": "Reviewed microservices architecture and agreed on gRPC for internal comms.",
					"action_items": ["Draft proto spec", "Benchmark latency"]
				}
			},
			"transcript": "Meeting recording transcript text"
		}
	}`)

	payload, err := ParseAnarlogPayload(raw, "")
	if err != nil {
		t.Fatalf("unexpected error parsing payload: %v", err)
	}

	if payload.Data.Meeting.Title != "Nested Architecture Review" {
		t.Errorf("expected title 'Nested Architecture Review', got: %q", payload.Data.Meeting.Title)
	}
	if len(payload.Data.Meeting.Summaries) != 1 || payload.Data.Meeting.Summaries[0] != "Reviewed microservices architecture and agreed on gRPC for internal comms." {
		t.Errorf("expected 1 summary item, got: %v", payload.Data.Meeting.Summaries)
	}
	if len(payload.Data.Meeting.ActionItems) != 2 {
		t.Errorf("expected 2 action items, got: %v", payload.Data.Meeting.ActionItems)
	}
	if payload.Data.TranscriptText != "Meeting recording transcript text" {
		t.Errorf("expected transcript text, got: %q", payload.Data.TranscriptText)
	}
}

func TestParseAnarlogPayload_RealPayload(t *testing.T) {
	raw := []byte(`{
		"created_at": "2026-09-06T23:08:45.495Z",
		"data": {
			"meeting": {
				"action_items": [],
				"created_at": "2026-09-06T23:06:29.340Z",
				"ended_at": "",
				"id": "a0e4d2a9-727d-4534-8230-65fc1ce3c0de",
				"kind": "meeting",
				"language": "",
				"note": {
					"created_at": "2026-09-06T23:06:29.340Z",
					"id": "a0e4d2a9-727d-4534-8230-65fc1ce3c0de",
					"kind": "note",
					"markdown": "memo 1\n\nmemo 2",
					"sort_order": 0,
					"template_id": "",
					"title": "",
					"updated_at": "2026-09-06T23:07:45.911Z"
				},
				"participants": [
					{
						"display_name": "",
						"email": "",
						"human_id": "bd9b07df-2b36-4488-ac6c-77e925770e1b",
						"job_title": "",
						"organization_id": "",
						"organization_name": "",
						"role": ""
					}
				],
				"series_id": "",
				"started_at": "",
				"status": "active",
				"summaries": [
					{
						"created_at": "2026-09-06T23:08:30.096Z",
						"id": "b04ffafa-8e72-4eae-87e4-1f8bece1b31c",
						"kind": "summary",
						"markdown": "# Audio Echo Testing\n\n# Audio and Echo Testing\n\n- The speaker investigated an unexplained audio echo occurring during a Google Meet recording.\n- Testing the microphone's Voice Isolation mode showed improved audio results.\n- The microphone setting was returned to standard mode after testing concluded.",
						"sort_order": 1,
						"template_id": "",
						"title": "Summary",
						"updated_at": "2026-09-06T23:08:45.488Z"
					}
				],
				"timezone": "",
				"title": "Audio Echo Testing",
				"updated_at": "2026-09-06T23:08:45.488Z"
			},
			"transcript_text": "Что ж, начинаем записывать."
		},
		"event": "note.enhanced",
		"id": "evt_ba87a6bc11bf4606bd0607c55b2b208a"
	}`)

	payload, err := ParseAnarlogPayload(raw, "")
	if err != nil {
		t.Fatalf("unexpected error parsing real payload: %v", err)
	}

	if payload.Data.Meeting.Title != "Audio Echo Testing" {
		t.Errorf("expected title 'Audio Echo Testing', got: %q", payload.Data.Meeting.Title)
	}
	if len(payload.Data.Meeting.Summaries) != 1 {
		t.Fatalf("expected 1 summary, got: %d", len(payload.Data.Meeting.Summaries))
	}
	if !strings.Contains(payload.Data.Meeting.Summaries[0], "The speaker investigated") {
		t.Errorf("expected summary markdown content, got: %q", payload.Data.Meeting.Summaries[0])
	}
	if len(payload.Data.Meeting.Participants) != 0 {
		t.Errorf("expected 0 participants (empty display_name), got: %v", payload.Data.Meeting.Participants)
	}
	if payload.Data.Meeting.Note != "memo 1\n\nmemo 2" {
		t.Errorf("expected note 'memo 1\\n\\nmemo 2', got: %q", payload.Data.Meeting.Note)
	}

	md := FormatMeetingMarkdown(payload)
	if !strings.Contains(md, "The speaker investigated an unexplained audio echo") {
		t.Errorf("expected markdown to contain summary content, got:\n%s", md)
	}
	if !strings.Contains(md, "memo 1\n\nmemo 2") {
		t.Errorf("expected markdown to contain notes, got:\n%s", md)
	}
	if !strings.Contains(md, "**Participants:** _None_") {
		t.Errorf("expected Participants: _None_, got:\n%s", md)
	}
	if !strings.Contains(md, "<!-- anarlog_meeting_id: a0e4d2a9-727d-4534-8230-65fc1ce3c0de -->") {
		t.Errorf("expected markdown to contain meeting ID comment, got:\n%s", md)
	}
}


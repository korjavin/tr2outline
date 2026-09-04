package main

import (
	"fmt"
	"strings"
	"time"
)

// FormatDocumentTitle formats the Outline document title.
// Format: "Meeting: {meeting.title} ({YYYY-MM-DD})"
func FormatDocumentTitle(meetingTitle, createdAt string) string {
	datePart := formatDateForTitle(createdAt)

	title := strings.TrimSpace(meetingTitle)
	if title == "" {
		title = "Untitled"
	}

	if datePart != "" {
		return fmt.Sprintf("Meeting: %s (%s)", title, datePart)
	}
	return fmt.Sprintf("Meeting: %s", title)
}

// FormatMeetingMarkdown generates the Markdown content for Outline.
func FormatMeetingMarkdown(payload *AnarlogWebhookPayload) string {
	meeting := payload.Data.Meeting

	formattedDate := formatCreatedAt(payload.CreatedAt)

	// Participants
	var participantsStr string
	if len(meeting.Participants) > 0 {
		participantsStr = strings.Join(meeting.Participants, ", ")
	} else {
		participantsStr = "_None_"
	}

	// Summaries
	var summaryBlock strings.Builder
	if len(meeting.Summaries) > 0 {
		for i, s := range meeting.Summaries {
			if i > 0 {
				summaryBlock.WriteString("\n")
			}
			summaryBlock.WriteString(fmt.Sprintf("- %s", strings.TrimSpace(s)))
		}
	} else {
		summaryBlock.WriteString("_No summary provided._")
	}

	// Action Items
	var actionItemsBlock strings.Builder
	if len(meeting.ActionItems) > 0 {
		for i, item := range meeting.ActionItems {
			if i > 0 {
				actionItemsBlock.WriteString("\n")
			}
			actionItemsBlock.WriteString(fmt.Sprintf("- [ ] %s", strings.TrimSpace(item)))
		}
	} else {
		actionItemsBlock.WriteString("_No action items._")
	}

	// Notes
	notes := strings.TrimSpace(meeting.Note)
	if notes == "" {
		notes = "_No notes._"
	}

	// Transcript
	transcript := strings.TrimSpace(payload.Data.TranscriptText)
	if transcript == "" {
		transcript = "_No transcript available._"
	}

	meetingTitle := strings.TrimSpace(meeting.Title)
	if meetingTitle == "" {
		meetingTitle = "Untitled"
	}

	// Construct full Markdown according to the specified template
	return fmt.Sprintf(`# Meeting: %s
**Date:** %s
**Participants:** %s

## 📝 Summary
%s

## ✅ Action Items
%s

## 🗒️ Notes
%s

---
<details>
<summary><b>🎙️ Full Transcript</b></summary>

%s
</details>
`, meetingTitle, formattedDate, participantsStr, summaryBlock.String(), actionItemsBlock.String(), notes, transcript)
}

func parseTime(raw string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", raw)
}

func formatDateForTitle(raw string) string {
	if raw == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	t, err := parseTime(raw)
	if err != nil {
		if len(raw) >= 10 {
			return raw[:10]
		}
		return raw
	}
	return t.UTC().Format("2006-01-02")
}

func formatCreatedAt(raw string) string {
	if raw == "" {
		return time.Now().UTC().Format("2006-01-02 15:04:05 MST")
	}
	t, err := parseTime(raw)
	if err != nil {
		return raw
	}
	return t.UTC().Format("2006-01-02 15:04:05 MST")
}

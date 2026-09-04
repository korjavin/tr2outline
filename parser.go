package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseAnarlogPayload parses raw JSON body into AnarlogWebhookPayload in a resilient way,
// handling type variations (note as object, numbers as strings, object arrays in action_items/summaries/participants, etc.).
func ParseAnarlogPayload(raw []byte, fallbackTimestamp string) (*AnarlogWebhookPayload, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("invalid JSON syntax: %w", err)
	}

	payload := &AnarlogWebhookPayload{}

	if id, ok := root["id"].(string); ok {
		payload.ID = id
	} else if idNum, ok := root["id"].(float64); ok {
		payload.ID = fmt.Sprintf("%.0f", idNum)
	}

	if event, ok := root["event"].(string); ok {
		payload.Event = event
	} else if eventType, ok := root["type"].(string); ok {
		payload.Event = eventType
	}

	// CreatedAt parsing (can be string or numeric unix timestamp)
	payload.CreatedAt = extractTimestampString(root["created_at"], fallbackTimestamp)

	// Extract data object
	dataMap, _ := root["data"].(map[string]interface{})
	if dataMap == nil {
		dataMap = root
	}

	// Extract transcript
	if t, ok := dataMap["transcript_text"].(string); ok {
		payload.Data.TranscriptText = t
	} else if t, ok := dataMap["transcript"].(string); ok {
		payload.Data.TranscriptText = t
	} else if segments, ok := dataMap["transcript"].([]interface{}); ok {
		payload.Data.TranscriptText = formatTranscriptSegments(segments)
	}

	// Extract meeting object
	meetingMap, _ := dataMap["meeting"].(map[string]interface{})
	if meetingMap == nil {
		meetingMap = dataMap
	}

	if id, ok := meetingMap["id"].(string); ok {
		payload.Data.Meeting.ID = id
	}
	if title, ok := meetingMap["title"].(string); ok {
		payload.Data.Meeting.Title = title
	}

	// Note extraction (handles string, object, sections)
	payload.Data.Meeting.Note = extractNoteString(meetingMap["note"])
	if payload.Data.Meeting.Note == "" {
		payload.Data.Meeting.Note = extractNoteString(meetingMap["notes"])
	}

	payload.Data.Meeting.Summaries = extractStringList(meetingMap["summaries"], "summary", "text", "content")
	if len(payload.Data.Meeting.Summaries) == 0 {
		payload.Data.Meeting.Summaries = extractStringList(meetingMap["summary"], "summary", "text", "content")
	}

	payload.Data.Meeting.ActionItems = extractStringList(meetingMap["action_items"], "task", "action", "text", "content", "title")
	if len(payload.Data.Meeting.ActionItems) == 0 {
		payload.Data.Meeting.ActionItems = extractStringList(meetingMap["actions"], "task", "action", "text", "content", "title")
	}

	payload.Data.Meeting.Participants = extractStringList(meetingMap["participants"], "name", "displayName", "email")

	return payload, nil
}

func extractNoteString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		// 1. Common single string field names in note object
		for _, key := range []string{"content", "text", "markdown", "body", "summary", "notes", "raw", "description"} {
			if str, ok := v[key].(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
		// 2. Structured sections (e.g. {"agenda": "...", "discussion": "..."})
		var sb strings.Builder
		for k, val := range v {
			if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
				sectionTitle := strings.Title(strings.ReplaceAll(k, "_", " "))
				sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", sectionTitle, strings.TrimSpace(str)))
			}
		}
		if sb.Len() > 0 {
			return strings.TrimSpace(sb.String())
		}
		// 3. Fallback: Pretty-printed JSON representation
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return fmt.Sprintf("%v", val)
}

func extractTimestampString(val interface{}, fallback string) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		sec := int64(v)
		if sec > 1e11 { // milliseconds
			return time.UnixMilli(sec).UTC().Format(time.RFC3339)
		}
		return time.Unix(sec, 0).UTC().Format(time.RFC3339)
	}

	if fallback != "" {
		if sec, err := strconv.ParseInt(fallback, 10, 64); err == nil {
			if sec > 1e11 {
				return time.UnixMilli(sec).UTC().Format(time.RFC3339)
			}
			return time.Unix(sec, 0).UTC().Format(time.RFC3339)
		}
		return fallback
	}

	return time.Now().UTC().Format(time.RFC3339)
}

func extractStringList(val interface{}, preferredKeys ...string) []string {
	if val == nil {
		return nil
	}

	var result []string

	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			switch elem := item.(type) {
			case string:
				if s := strings.TrimSpace(elem); s != "" {
					result = append(result, s)
				}
			case map[string]interface{}:
				found := false
				for _, key := range preferredKeys {
					if text, ok := elem[key].(string); ok && strings.TrimSpace(text) != "" {
						result = append(result, strings.TrimSpace(text))
						found = true
						break
					}
				}
				if !found {
					for _, val := range elem {
						if text, ok := val.(string); ok && strings.TrimSpace(text) != "" {
							result = append(result, strings.TrimSpace(text))
							break
						}
					}
				}
			}
		}
	case string:
		for _, line := range strings.Split(v, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" {
				result = append(result, line)
			}
		}
	}

	return result
}

func formatTranscriptSegments(segments []interface{}) string {
	var sb strings.Builder
	for _, item := range segments {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		speaker, _ := m["speaker"].(string)
		text, _ := m["text"].(string)
		if text == "" {
			text, _ = m["content"].(string)
		}
		if text == "" {
			continue
		}
		if speaker != "" {
			sb.WriteString(fmt.Sprintf("**%s:** %s\n\n", speaker, text))
		} else {
			sb.WriteString(fmt.Sprintf("%s\n\n", text))
		}
	}
	return strings.TrimSpace(sb.String())
}

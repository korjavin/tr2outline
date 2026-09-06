package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
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

	// Extract meeting object
	meetingMap, _ := dataMap["meeting"].(map[string]interface{})
	if meetingMap == nil {
		meetingMap = dataMap
	}

	// Extract note object if note is a map
	var noteMap map[string]interface{}
	if m, ok := meetingMap["note"].(map[string]interface{}); ok {
		noteMap = m
	} else if m, ok := meetingMap["notes"].(map[string]interface{}); ok {
		noteMap = m
	} else if m, ok := dataMap["note"].(map[string]interface{}); ok {
		noteMap = m
	}

	// Log structure keys so operator can inspect exact schema in Portainer
	log.Printf("[Webhook Schema] root_keys=%v, data_keys=%v, meeting_keys=%v, note_keys=%v",
		mapKeys(root), mapKeys(dataMap), mapKeys(meetingMap), mapKeys(noteMap))

	// Truncated payload snippet in logs
	rawSnippet := string(raw)
	if len(rawSnippet) > 2000 {
		rawSnippet = rawSnippet[:2000] + "... (truncated)"
	}
	log.Printf("[Webhook Payload] %s", rawSnippet)

	if id, ok := meetingMap["id"].(string); ok {
		payload.Data.Meeting.ID = id
	}

	// Title extraction: check meeting, note, data, root
	payload.Data.Meeting.Title = firstNonEmptyString(
		getString(meetingMap, "title"),
		getString(noteMap, "title"),
		getString(dataMap, "title"),
		getString(root, "title"),
		getString(meetingMap, "topic"),
		getString(noteMap, "topic"),
		getString(meetingMap, "name"),
		getString(noteMap, "name"),
	)

	// Summaries: check meeting, note, data, root for summaries/summary/key_points/takeaways/highlights/overview
	summarySources := []interface{}{
		meetingMap["summaries"], meetingMap["summary"],
		noteMap["summaries"], noteMap["summary"],
		dataMap["summaries"], dataMap["summary"],
		root["summaries"], root["summary"],
		meetingMap["key_points"], noteMap["key_points"], dataMap["key_points"],
		meetingMap["takeaways"], noteMap["takeaways"], dataMap["takeaways"],
		meetingMap["highlights"], noteMap["highlights"], dataMap["highlights"],
		meetingMap["overview"], noteMap["overview"], dataMap["overview"],
	}
	for _, src := range summarySources {
		if list := extractStringList(src, "markdown", "summary", "text", "content", "point"); len(list) > 0 {
			payload.Data.Meeting.Summaries = list
			break
		}
	}

	// Action items: check meeting, note, data, root for action_items/actions/todos/tasks/next_steps
	actionSources := []interface{}{
		meetingMap["action_items"], meetingMap["actions"],
		noteMap["action_items"], noteMap["actions"],
		dataMap["action_items"], dataMap["actions"],
		root["action_items"], root["actions"],
		meetingMap["todos"], noteMap["todos"], dataMap["todos"],
		meetingMap["tasks"], noteMap["tasks"], dataMap["tasks"],
		meetingMap["next_steps"], noteMap["next_steps"], dataMap["next_steps"],
	}
	for _, src := range actionSources {
		if list := extractStringList(src, "markdown", "task", "action", "text", "content", "item", "description", "title"); len(list) > 0 {
			payload.Data.Meeting.ActionItems = list
			break
		}
	}

	// Participants: check meeting, note, data
	participantSources := []interface{}{
		meetingMap["participants"], meetingMap["attendees"],
		noteMap["participants"], noteMap["attendees"],
		dataMap["participants"], dataMap["attendees"],
		meetingMap["members"], dataMap["members"],
	}
	for _, src := range participantSources {
		if list := extractParticipants(src); len(list) > 0 {
			payload.Data.Meeting.Participants = list
			break
		}
	}

	// Note extraction
	if str := getString(meetingMap, "note"); str != "" {
		payload.Data.Meeting.Note = str
	} else if str := getString(meetingMap, "notes"); str != "" {
		payload.Data.Meeting.Note = str
	} else if noteMap != nil {
		payload.Data.Meeting.Note = extractNoteString(noteMap)
	} else if str := getString(dataMap, "note"); str != "" {
		payload.Data.Meeting.Note = str
	}

	// Transcript extraction
	if t, ok := dataMap["transcript_text"].(string); ok && strings.TrimSpace(t) != "" {
		payload.Data.TranscriptText = t
	} else if t, ok := dataMap["transcript"].(string); ok && strings.TrimSpace(t) != "" {
		payload.Data.TranscriptText = t
	} else if segments, ok := dataMap["transcript"].([]interface{}); ok && len(segments) > 0 {
		payload.Data.TranscriptText = formatTranscriptSegments(segments)
	} else if t, ok := root["transcript_text"].(string); ok && strings.TrimSpace(t) != "" {
		payload.Data.TranscriptText = t
	}

	return payload, nil
}

func mapKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key].(string); ok {
		return strings.TrimSpace(val)
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func extractNoteString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		// 1. Common single string field names in note object (excluding summaries/title if possible)
		for _, key := range []string{"content", "text", "markdown", "body", "notes", "raw", "description"} {
			if str, ok := v[key].(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
		// 2. Structured sections (e.g. {"agenda": "...", "discussion": "..."})
		var sb strings.Builder
		for k, val := range v {
			// Skip title, summary, action_items if they are rendered in their own dedicated sections
			if k == "title" || k == "summary" || k == "summaries" || k == "action_items" || k == "actions" || k == "participants" {
				continue
			}
			if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
				sectionTitle := strings.Title(strings.ReplaceAll(k, "_", " "))
				sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", sectionTitle, strings.TrimSpace(str)))
			}
		}
		if sb.Len() > 0 {
			return strings.TrimSpace(sb.String())
		}
		// 3. If only summary exists inside note
		if str, ok := v["summary"].(string); ok && strings.TrimSpace(str) != "" {
			return strings.TrimSpace(str)
		}
		// 4. Fallback: Pretty-printed JSON representation
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
				if !found && len(preferredKeys) == 0 {
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

func extractParticipants(val interface{}) []string {
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
				for _, key := range []string{"display_name", "displayName", "name", "full_name", "email", "username"} {
					if text, ok := elem[key].(string); ok && strings.TrimSpace(text) != "" {
						result = append(result, strings.TrimSpace(text))
						break
					}
				}
			}
		}
	case string:
		for _, line := range strings.Split(v, ",") {
			if s := strings.TrimSpace(line); s != "" {
				result = append(result, s)
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

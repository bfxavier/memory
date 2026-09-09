package extractor

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bfxavier/memory/internal/model"
)

func renderInput(events []model.Event, active []model.Memory, maxBytes int) (string, map[string]bool, map[string]bool) {
	eventText, eventIDs := renderEvents(events, maxBytes*3/4)
	activeText, activeIDs := renderActiveMemories(active, maxBytes-len(eventText))
	if eventText == "" {
		return "", eventIDs, activeIDs
	}
	return activeText + eventText, eventIDs, activeIDs
}

func renderEvents(events []model.Event, maxBytes int) (string, map[string]bool) {
	selected := []string{}
	ids := map[string]bool{}
	size := 0
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		line := fmt.Sprintf("EVENT %s %s %s\n%s\n", event.ID, event.EventName, event.CreatedAt.Format(time.RFC3339Nano), event.Payload)
		if len(line) > maxBytes {
			continue
		}
		if size+len(line) > maxBytes {
			break
		}
		selected = append(selected, line)
		size += len(line)
		ids[event.ID] = true
	}
	if len(selected) == 0 {
		return "", ids
	}
	var rendered strings.Builder
	rendered.WriteString("NEW EVENTS\n")
	for index := len(selected) - 1; index >= 0; index-- {
		rendered.WriteString(selected[index])
	}
	return rendered.String(), ids
}

func renderActiveMemories(memories []model.Memory, maxBytes int) (string, map[string]bool) {
	ids := map[string]bool{}
	if len(memories) == 0 || maxBytes <= 0 {
		return "", ids
	}
	var rendered strings.Builder
	rendered.WriteString("EXISTING ACTIVE MEMORIES\n")
	for _, memory := range memories {
		line := fmt.Sprintf("MEMORY %s [%s] %s\n", memory.ID, memory.Kind, strings.ReplaceAll(memory.Content, "\n", " "))
		if rendered.Len()+len(line) > maxBytes {
			break
		}
		rendered.WriteString(line)
		ids[memory.ID] = true
	}
	return rendered.String(), ids
}

func clip(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

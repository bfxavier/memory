package extractor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bfxavier/memory/internal/model"
)

const systemPrompt = `Extract durable memory from coding-agent events.
Return JSON only: {"memories":[{"kind":"decision|fact|preference|procedure|failure|outcome|note","content":"...","confidence":0.0,"tags":["..."],"source_event_id":"...","supersedes_id":""}]}.
Keep only information useful in a future session: explicit decisions and reasons, confirmed facts, recurring procedures, user preferences, failed approaches with causes, and completed outcomes.
Reject transient task state, raw logs, secrets, guesses, plans not adopted, assistant claims without evidence, and facts obvious from repository contents.
Use concise standalone statements. Do not emit duplicates. Every source_event_id must match an input event ID.
Existing active memories are historical evidence. When a new memory directly replaces or contradicts one, set supersedes_id to that active memory ID. Otherwise set supersedes_id to an empty string. Never supersede unrelated or merely more detailed memories. Return an empty memories array when nothing qualifies.`

type extractionResponse struct {
	Memories []model.ExtractedMemory `json:"memories"`
}

func parseExtraction(content string, eventIDs, activeIDs map[string]bool, maxMemories int) ([]model.ExtractedMemory, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var extracted extractionResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &extracted); err != nil {
		return nil, fmt.Errorf("decode extraction: %w", err)
	}
	if len(extracted.Memories) > maxMemories {
		return nil, fmt.Errorf("provider returned %d memories, maximum is %d", len(extracted.Memories), maxMemories)
	}
	for index := range extracted.Memories {
		if err := validateExtractedMemory(&extracted.Memories[index], eventIDs, activeIDs); err != nil {
			return nil, fmt.Errorf("memory at index %d: %w", index, err)
		}
	}
	return extracted.Memories, nil
}

func validateExtractedMemory(memory *model.ExtractedMemory, eventIDs, activeIDs map[string]bool) error {
	memory.Content = strings.TrimSpace(memory.Content)
	if !validKind(memory.Kind) || memory.Content == "" || len(memory.Content) > 2000 {
		return fmt.Errorf("invalid kind or content")
	}
	if memory.Confidence < 0.5 || memory.Confidence > 1 {
		return fmt.Errorf("invalid confidence")
	}
	if !eventIDs[memory.SourceEventID] {
		return fmt.Errorf("invalid source_event_id %q", memory.SourceEventID)
	}
	if len(memory.Tags) > 16 {
		return fmt.Errorf("too many tags")
	}
	if memory.SupersedesID != "" && !activeIDs[memory.SupersedesID] {
		return fmt.Errorf("invalid supersedes_id %q", memory.SupersedesID)
	}
	return nil
}

func validKind(value string) bool {
	switch value {
	case "decision", "fact", "preference", "procedure", "failure", "outcome", "note":
		return true
	default:
		return false
	}
}

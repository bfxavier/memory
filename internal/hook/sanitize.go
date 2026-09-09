package hook

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxStringBytes = 8 * 1024
	maxArrayItems  = 64
	maxDepth       = 8
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{16,}\b`),
	regexp.MustCompile(`(?i)\bgh[opsu]_[a-z0-9]{20,}\b`),
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b(bearer)\s+[a-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`(?im)^([a-z0-9_]*(?:token|secret|password|api_?key)[a-z0-9_]*)\s*=\s*[^\r\n]+`),
}

func sanitizePayload(raw map[string]any) any {
	copy := make(map[string]any, len(raw))
	for key, value := range raw {
		copy[key] = value
	}
	if containsSensitivePath(raw["tool_input"]) {
		copy["tool_response"] = "[REDACTED]"
		copy["tool_output"] = "[REDACTED]"
	}
	return sanitize(copy, "", 0)
}

func sanitize(value any, key string, depth int) any {
	if depth > maxDepth {
		return "[TRUNCATED]"
	}
	if sensitiveKey(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			if droppedKey(childKey) {
				continue
			}
			output[childKey] = sanitize(childValue, childKey, depth+1)
		}
		return output
	case []any:
		limit := len(typed)
		if limit > maxArrayItems {
			limit = maxArrayItems
		}
		output := make([]any, 0, limit)
		for _, child := range typed[:limit] {
			output = append(output, sanitize(child, key, depth+1))
		}
		return output
	case string:
		return sanitizeString(typed)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, term := range []string{"api_key", "apikey", "authorization", "credential", "password", "private_key", "secret", "token"} {
		if strings.Contains(key, term) {
			return true
		}
	}
	return false
}

func droppedKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "base64") || strings.Contains(key, "image_data") || strings.Contains(key, "screenshot")
}

func sanitizeString(value string) string {
	for _, pattern := range secretPatterns {
		value = pattern.ReplaceAllString(value, "[REDACTED]")
	}
	if len(value) <= maxStringBytes {
		return value
	}
	value = value[:maxStringBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + "\n[TRUNCATED]"
}

func containsSensitivePath(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.Contains(strings.ToLower(key), "path") {
				if text, ok := child.(string); ok && sensitivePath(text) {
					return true
				}
			}
			if containsSensitivePath(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSensitivePath(child) {
				return true
			}
		}
	}
	return false
}

func sensitivePath(value string) bool {
	value = strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
	base := value
	if index := strings.LastIndex(value, "/"); index >= 0 {
		base = value[index+1:]
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	for _, term := range []string{"credential", "secrets", "id_rsa", "id_ed25519"} {
		if strings.Contains(base, term) {
			return true
		}
	}
	return false
}

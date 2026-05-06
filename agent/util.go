package agent

import "strings"

const DefaultMaxMessageLength = 2048

func TruncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func IsClaudeCodeConfirmationBlock(text string) bool {
	lower := strings.ToLower(text)
	markers := []string{
		"permission",
		"confirm",
		"approval",
		"requires confirmation",
		"do you want to proceed",
		"waiting for",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

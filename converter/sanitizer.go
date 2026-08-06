package converter

import (
	"regexp"
	"strings"
	"unicode"
)

// CleanIdentifier cleans identifiers while preserving Unicode (e.g. Chinese) characters in Python 3.
func CleanIdentifier(text string, defaultName string) string {
	if defaultName == "" {
		defaultName = "identifier"
	}
	text = strings.TrimSpace(text)
	if text != "" && IsValidIdentifier(text) {
		return text
	}

	// Remove leading numbers or underscores
	reLeading := regexp.MustCompile(`^[0-9_]+`)
	cleaned := reLeading.ReplaceAllString(text, "")

	var sb strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := sb.String()
	reUnderscores := regexp.MustCompile(`_+`)
	res = reUnderscores.ReplaceAllString(res, "_")
	res = strings.Trim(res, "_")

	if res == "" || !IsValidIdentifier(res) {
		return defaultName
	}
	return res
}

func IsValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return true
}

func QuoteString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}

func QuoteField(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `'` + s + `'`
}

package converter

import "testing"

func TestCleanIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		defVal   string
		expected string
	}{
		{"Stage", "target", "Stage"},
		{"玩家", "target", "玩家"},
		{"角色1", "target", "角色1"},
		{"123abc", "target", "abc"},
		{"var with spaces", "var", "var_with_spaces"},
		{"", "default", "default"},
		{"玩家-血量!", "var", "玩家_血量"},
	}

	for _, tt := range tests {
		got := CleanIdentifier(tt.input, tt.defVal)
		if got != tt.expected {
			t.Errorf("CleanIdentifier(%q, %q) = %q; want %q", tt.input, tt.defVal, got, tt.expected)
		}
	}
}

func TestQuoteString(t *testing.T) {
	if got := QuoteString(`hello "world"`); got != `"hello \"world\""` {
		t.Errorf("QuoteString failed: %s", got)
	}
}

func TestQuoteField(t *testing.T) {
	if got := QuoteField(`it's fine`); got != `'it\'s fine'` {
		t.Errorf("QuoteField failed: %s", got)
	}
}

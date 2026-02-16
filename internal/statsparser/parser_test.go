package statsparser

import "testing"

func TestParseCount(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1.2K", 1200},
		{"1.5M", 1500000},
		{"123", 123},
		{"12,345", 12345},
		{"1 234", 1234},
		{"5.6K views", 5600},
		{"100K", 100000},
		{"2.3M", 2300000},
		{"0", 0},
		{"", 0},
		{"no number", 0},
		{"42k", 42000},
		{"3.14k", 3140},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCount(tt.input)
			if result != tt.expected {
				t.Errorf("parseCount(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGuessLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Привет мир, это тестовый текст на русском языке", "ru"},
		{"Hello world, this is a test text in English", "en"},
		{"", "unknown"},
		{"مرحبا بالعالم", "ar"},
		{"12345 !!!", "unknown"},
		// Mixed — cyrillic dominant
		{"Привет hello мир world тест test текст text", "ru"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := guessLanguage(tt.input)
			if result != tt.expected {
				t.Errorf("guessLanguage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

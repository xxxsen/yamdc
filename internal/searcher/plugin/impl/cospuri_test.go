package impl

import (
	"testing"
)

func TestExtractModelAndID(t *testing.T) {
	plg := &cospuri{}

	tests := []struct {
		number        string
		expectedModel string
		expectedID    string
		expectError   bool
	}{
		{"COSPURI-Emiri-Momota-0548cpar", "Emiri-Momota", "0548cpar", false},
		{"COSPURI-Emiri-Momota-0548", "Emiri-Momota", "0548", false},
		{"COSPURI-Invalid-Format", "", "", true},
		{"cospuri-Emiri-Momota-0548", "Emiri-Momota", "0548", false},
		{"cospuri-AAAA-BBBBB-CCCCC-DDDDD-22223aaa", "Aaaa-Bbbbb-Ccccc-Ddddd", "22223aaa", false},
		{"cospuri-AAAA-BBBBB-CCCCC-DDDDD-22223", "Aaaa-Bbbbb-Ccccc-Ddddd", "22223", false},
		{"cospuri-12345abc", "", "12345abc", false},
	}

	for _, test := range tests {
		model, id, err := plg.extractModelAndID(test.number)
		if (err != nil) != test.expectError {
			t.Errorf("extractModelAndID(%q) error = %v; want error = %v", test.number, err != nil, test.expectError)
		}
		if model != test.expectedModel || id != test.expectedID {
			t.Errorf("extractModelAndID(%q) = (%q, %q); want (%q, %q)", test.number, model, id, test.expectedModel, test.expectedID)
		}
	}
}

func TestNormalizeModel(t *testing.T) {
	plg := &cospuri{}

	tests := []struct {
		input    string
		expected string
	}{
		{"emiri-momota", "Emiri-Momota"},
		{"EMIRI-MOMOTA", "Emiri-Momota"},
		{"eMiRi-MoMoTa", "Emiri-Momota"},
		{"singleword", "Singleword"},
	}

	for _, test := range tests {
		result := plg.normalizeModel(test.input)
		if result != test.expected {
			t.Errorf("normalizeModel(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

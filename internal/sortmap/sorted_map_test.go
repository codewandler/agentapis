package sortmap

import (
	"encoding/json"
	"testing"
)

func TestNewSortedMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: "{}",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "{}",
		},
		{
			name: "simple map",
			input: map[string]any{
				"c": 3,
				"a": 1,
				"b": 2,
			},
			expected: `{"a":1,"b":2,"c":3}`,
		},
		{
			name: "nested map",
			input: map[string]any{
				"z": map[string]any{
					"y": 1,
					"x": 2,
				},
				"a": 1,
			},
			expected: `{"a":1,"z":{"x":2,"y":1}}`,
		},
		{
			name: "with arrays",
			input: map[string]any{
				"b": []any{3, 1, 2},
				"a": 1,
			},
			expected: `{"a":1,"b":[3,1,2]}`,
		},
		{
			name: "nested arrays and maps",
			input: map[string]any{
				"z": map[string]any{
					"y": []any{map[string]any{"nested": "value"}},
				},
				"a": 1,
			},
			expected: `{"a":1,"z":{"y":[{"nested":"value"}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSortedMap(tt.input)
			got, err := json.Marshal(sm)
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(got))
			}
		})
	}
}

func TestMarshalJSONPropagatesValueErrors(t *testing.T) {
	sm := NewSortedMap(map[string]any{"bad": make(chan int)})

	_, err := json.Marshal(sm)
	if err == nil {
		t.Fatal("expected marshal error for unsupported value type")
	}
}

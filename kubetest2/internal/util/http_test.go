package util

import (
	"testing"
)

func Test_NewHTTPHeaderAPIOptions(t *testing.T) {
	testCases := []struct {
		name        string
		headers     []string
		expectError bool
	}{
		{
			name:    "empty",
			headers: []string{},
		},
		{
			name:    "single valid header",
			headers: []string{"Content-Type: application/json"},
		},
		{
			name:    "multiple valid headers",
			headers: []string{"Content-Type: application/json", "Accept: application/json"},
		},
		{
			name:        "invalid header",
			headers:     []string{"Invalid header"},
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewHTTPHeaderAPIOptions(tc.headers)
			if err != nil && !tc.expectError {
				t.Errorf("unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Error("expected error but got none")
			}
		})
	}
}

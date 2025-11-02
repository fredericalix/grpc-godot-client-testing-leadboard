package service

import (
	"testing"
)

func TestValidatePlayerName(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid name",
			input:     "Alice",
			wantError: false,
		},
		{
			name:      "max length name",
			input:     "12345678901234567890", // exactly 20 chars
			wantError: false,
		},
		{
			name:      "too long name",
			input:     "123456789012345678901", // 21 chars
			wantError: true,
		},
		{
			name:      "empty name",
			input:     "",
			wantError: true,
		},
		{
			name:      "single character",
			input:     "A",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.validatePlayerName(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("validatePlayerName(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestValidateScore(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name      string
		input     int64
		wantError bool
	}{
		{
			name:      "valid score zero",
			input:     0,
			wantError: false,
		},
		{
			name:      "valid positive score",
			input:     100,
			wantError: false,
		},
		{
			name:      "valid large score",
			input:     999999999,
			wantError: false,
		},
		{
			name:      "invalid negative score",
			input:     -1,
			wantError: true,
		},
		{
			name:      "invalid large negative score",
			input:     -999999,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.validateScore(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("validateScore(%d) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestMaxPlayerNameLength(t *testing.T) {
	// Ensure the constant matches requirements
	if MaxPlayerNameLength != 20 {
		t.Errorf("MaxPlayerNameLength = %d, want 20", MaxPlayerNameLength)
	}
}

func TestMinPlayerNameLength(t *testing.T) {
	// Ensure the constant matches requirements
	if MinPlayerNameLength != 1 {
		t.Errorf("MinPlayerNameLength = %d, want 1", MinPlayerNameLength)
	}
}

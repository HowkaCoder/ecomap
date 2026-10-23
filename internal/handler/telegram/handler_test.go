package telegram

import (
	"testing"

	"ecomap/internal/domain/bin"
)

func TestParseBinCallback(t *testing.T) {
	tests := []struct {
		data   string
		id     uint64
		status string
		ok     bool
	}{
		{"bin_full_12", 12, bin.StatusFull, true},
		{"bin_empty_7", 7, bin.StatusEmpty, true},
		{"bin_full_", 0, "", false},
		{"something", 0, "", false},
	}

	for _, tt := range tests {
		id, status, ok := parseBinCallback(tt.data)
		if id != tt.id || status != tt.status || ok != tt.ok {
			t.Errorf("parseBinCallback(%q) = %d, %q, %v", tt.data, id, status, ok)
		}
	}
}

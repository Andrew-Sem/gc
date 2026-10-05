package domain

import "testing"

func TestValidFullName(t *testing.T) {
	tests := []struct {
		fullName string
		want     bool
	}{
		{"acme/api", true},
		{"acmeapi", false},
		{"/api", false},
		{"acme/", false},
		{"acme/api/extra", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			if got := ValidFullName(tt.fullName); got != tt.want {
				t.Errorf("ValidFullName(%q) = %v, want %v", tt.fullName, got, tt.want)
			}
		})
	}
}

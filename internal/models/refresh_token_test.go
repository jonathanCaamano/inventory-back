package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRefreshToken_BeforeCreate_AssignsUUID(t *testing.T) {
	tok := &RefreshToken{}
	if err := tok.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.ID == uuid.Nil {
		t.Error("expected UUID to be assigned")
	}
}

func TestRefreshToken_BeforeCreate_KeepsExistingUUID(t *testing.T) {
	existing := uuid.New()
	tok := &RefreshToken{ID: existing}
	if err := tok.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.ID != existing {
		t.Errorf("expected UUID to remain %s, got %s", existing, tok.ID)
	}
}

func TestRefreshToken_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		revoked   bool
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "valid token",
			revoked:   false,
			expiresAt: time.Now().Add(time.Hour),
			expected:  true,
		},
		{
			name:      "revoked token",
			revoked:   true,
			expiresAt: time.Now().Add(time.Hour),
			expected:  false,
		},
		{
			name:      "expired token",
			revoked:   false,
			expiresAt: time.Now().Add(-time.Hour),
			expected:  false,
		},
		{
			name:      "revoked and expired",
			revoked:   true,
			expiresAt: time.Now().Add(-time.Hour),
			expected:  false,
		},
		{
			name:      "expires exactly now",
			revoked:   false,
			expiresAt: time.Now().Add(-time.Millisecond),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &RefreshToken{
				Revoked:   tt.revoked,
				ExpiresAt: tt.expiresAt,
			}
			if got := token.IsValid(); got != tt.expected {
				t.Errorf("IsValid()=%v, want %v", got, tt.expected)
			}
		})
	}
}

package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestUser_CanManage(t *testing.T) {
	tests := []struct {
		role     Role
		expected bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleViewer, false},
		{"unknown", false},
	}

	for _, tt := range tests {
		u := &User{Role: tt.role}
		if got := u.CanManage(); got != tt.expected {
			t.Errorf("role=%s: CanManage()=%v, want %v", tt.role, got, tt.expected)
		}
	}
}

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		role     Role
		expected bool
	}{
		{RoleAdmin, true},
		{RoleManager, false},
		{RoleViewer, false},
	}

	for _, tt := range tests {
		u := &User{Role: tt.role}
		if got := u.IsAdmin(); got != tt.expected {
			t.Errorf("role=%s: IsAdmin()=%v, want %v", tt.role, got, tt.expected)
		}
	}
}

func TestUser_BeforeCreate_AssignsUUID(t *testing.T) {
	u := &User{}
	if err := u.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == uuid.Nil {
		t.Error("expected UUID to be assigned")
	}
}

func TestUser_BeforeCreate_KeepsExistingUUID(t *testing.T) {
	existing := uuid.New()
	u := &User{ID: existing}
	if err := u.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != existing {
		t.Errorf("expected UUID to remain %s, got %s", existing, u.ID)
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("expected RoleAdmin='admin', got '%s'", RoleAdmin)
	}
	if RoleManager != "manager" {
		t.Errorf("expected RoleManager='manager', got '%s'", RoleManager)
	}
	if RoleViewer != "viewer" {
		t.Errorf("expected RoleViewer='viewer', got '%s'", RoleViewer)
	}
}

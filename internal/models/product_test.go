package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestProduct_BeforeCreate_AssignsUUID(t *testing.T) {
	p := &Product{}
	if err := p.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == uuid.Nil {
		t.Error("expected UUID to be assigned")
	}
}

func TestProduct_BeforeCreate_KeepsExistingUUID(t *testing.T) {
	existing := uuid.New()
	p := &Product{ID: existing}
	if err := p.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != existing {
		t.Errorf("expected UUID to remain %s, got %s", existing, p.ID)
	}
}

func TestCategory_BeforeCreate_AssignsUUID(t *testing.T) {
	c := &Category{}
	if err := c.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID == uuid.Nil {
		t.Error("expected UUID to be assigned")
	}
}

func TestCategory_BeforeCreate_KeepsExistingUUID(t *testing.T) {
	existing := uuid.New()
	c := &Category{ID: existing}
	if err := c.BeforeCreate(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != existing {
		t.Errorf("expected UUID to remain %s, got %s", existing, c.ID)
	}
}

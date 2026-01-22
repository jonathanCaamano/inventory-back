package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
	"github.com/jonathanCaamano/inventory-back/internal/domain/user"
)

type memUsers struct {
	byName map[string]uuid.UUID
}

func (m *memUsers) GetForLogin(ctx context.Context, username string) (uuid.UUID, string, bool, bool, error) {
	return uuid.Nil, "", false, false, errors.New("not_impl")
}
func (m *memUsers) Create(ctx context.Context, username, password string, isAdmin bool) (uuid.UUID, error) {
	if m.byName == nil {
		m.byName = map[string]uuid.UUID{}
	}
	if _, ok := m.byName[username]; ok {
		return uuid.Nil, errors.New("username_taken")
	}
	id := uuid.New()
	m.byName[username] = id
	return id, nil
}
func (m *memUsers) GetByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	return user.User{}, errors.New("not_impl")
}
func (m *memUsers) List(ctx context.Context, search string, limit, offset int) ([]user.User, int, error) {
	return nil, 0, errors.New("not_impl")
}

// --- groups ---

type memGroups struct {
	items   map[uuid.UUID]group.Group
	bySlug  map[string]uuid.UUID
	members map[[2]uuid.UUID]group.Role
}

func (m *memGroups) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]group.Group, error) {
	return nil, errors.New("not_impl")
}
func (m *memGroups) ListAll(ctx context.Context) ([]group.Group, error) {
	out := []group.Group{}
	for _, g := range m.items {
		out = append(out, g)
	}
	return out, nil
}
func (m *memGroups) Count(ctx context.Context) (int, error) {
	return len(m.items), nil
}
func (m *memGroups) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error) {
	g, ok := m.items[id]
	if !ok {
		return group.Group{}, errors.New("not_found")
	}
	return g, nil
}
func (m *memGroups) GetBySlug(ctx context.Context, slug string) (group.Group, error) {
	id, ok := m.bySlug[slug]
	if !ok {
		return group.Group{}, errors.New("not_found")
	}
	return m.items[id], nil
}
func (m *memGroups) Create(ctx context.Context, slug, name string) (group.Group, error) {
	if m.items == nil {
		m.items = map[uuid.UUID]group.Group{}
		m.bySlug = map[string]uuid.UUID{}
	}
	if _, ok := m.bySlug[slug]; ok {
		return group.Group{}, errors.New("slug_taken")
	}
	g := group.Group{ID: uuid.New(), Slug: slug, Name: name}
	m.items[g.ID] = g
	m.bySlug[slug] = g.ID
	return g, nil
}
func (m *memGroups) AddMember(ctx context.Context, groupID, userID uuid.UUID, role group.Role) error {
	if m.members == nil {
		m.members = map[[2]uuid.UUID]group.Role{}
	}
	m.members[[2]uuid.UUID{userID, groupID}] = role
	return nil
}
func (m *memGroups) ListMembers(ctx context.Context, groupID uuid.UUID) ([]group.Member, error) {
	return nil, errors.New("not_impl")
}
func (m *memGroups) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return errors.New("not_impl")
}
func (m *memGroups) RoleForUser(ctx context.Context, userID, groupID uuid.UUID) (group.Role, error) {
	return "", errors.New("not_impl")
}
func (m *memGroups) FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	return uuid.Nil, errors.New("not_impl")
}

func TestRegisterRequiresGroup(t *testing.T) {
	u := &memUsers{}
	g := &memGroups{items: map[uuid.UUID]group.Group{}, bySlug: map[string]uuid.UUID{}}
	svc := New(u, g, "secret", time.Hour)
	_, err := svc.Register(context.Background(), "firstuser", "S3curePass!", uuid.Nil, "", "")
	if err == nil || err.Error() != "group_required" {
		t.Fatalf("expected group_required, got %v", err)
	}
}

func TestRegisterCreatesNewGroupAsWriter(t *testing.T) {
	u := &memUsers{}
	g := &memGroups{items: map[uuid.UUID]group.Group{}, bySlug: map[string]uuid.UUID{}}
	svc := New(u, g, "secret", time.Hour)

	res, err := svc.Register(context.Background(), "firstuser", "S3curePass!", uuid.Nil, "", "Electrodomesticos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Group.ID == uuid.Nil {
		t.Fatalf("expected group created")
	}
	if res.Role != group.RoleWriter {
		t.Fatalf("expected writer role, got %s", res.Role)
	}
}

func TestRegisterUsesExistingGroupBySlug(t *testing.T) {
	u := &memUsers{}
	g := &memGroups{items: map[uuid.UUID]group.Group{}, bySlug: map[string]uuid.UUID{}}
	existing, _ := g.Create(context.Background(), "coches", "Coches")

	svc := New(u, g, "secret", time.Hour)
	res, err := svc.Register(context.Background(), "user1", "S3curePass!", uuid.Nil, "coches", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Group.ID != existing.ID {
		t.Fatalf("expected existing group")
	}
	if res.Role != group.RoleReader {
		t.Fatalf("expected reader role, got %s", res.Role)
	}
}

func TestRegisterCreatesNewGroupEvenIfGroupsExist(t *testing.T) {
	u := &memUsers{}
	g := &memGroups{items: map[uuid.UUID]group.Group{}, bySlug: map[string]uuid.UUID{}}
	_, _ = g.Create(context.Background(), "coches", "Coches")

	svc := New(u, g, "secret", time.Hour)
	res, err := svc.Register(context.Background(), "user1", "S3curePass!", uuid.Nil, "", "Motos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Group.Slug != "motos" {
		t.Fatalf("expected group slug motos, got %s", res.Group.Slug)
	}
	if res.Role != group.RoleWriter {
		t.Fatalf("expected writer role, got %s", res.Role)
	}
}

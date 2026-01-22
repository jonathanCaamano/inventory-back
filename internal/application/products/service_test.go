package products

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
	"github.com/jonathanCaamano/inventory-back/internal/domain/product"
)

type fakeProductRepo struct {
	gidByID map[uuid.UUID]uuid.UUID
}

func (f *fakeProductRepo) Create(ctx context.Context, p *product.Product) (*product.Product, error) {
	if f.gidByID == nil {
		f.gidByID = map[uuid.UUID]uuid.UUID{}
	}
	p.ID = uuid.New()
	f.gidByID[p.ID] = p.GroupID
	return p, nil
}
func (f *fakeProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	gid := f.gidByID[id]
	return &product.Product{ID: id, GroupID: gid, Name: "x", EntryDate: time.Now(), Status: product.StatusDraft}, nil
}
func (f *fakeProductRepo) Update(ctx context.Context, p *product.Product) (*product.Product, error) {
	return p, nil
}
func (f *fakeProductRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (f *fakeProductRepo) Search(ctx context.Context, q product.SearchQuery, isAdmin bool) ([]*product.Product, int, error) {
	return nil, 0, nil
}
func (f *fakeProductRepo) AddImage(ctx context.Context, productID uuid.UUID, img product.Image) (*product.Product, error) {
	return &product.Product{ID: productID}, nil
}
func (f *fakeProductRepo) UpsertContact(ctx context.Context, productID uuid.UUID, c product.Contact) (*product.Product, error) {
	return &product.Product{ID: productID}, nil
}
func (f *fakeProductRepo) GetGroupID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return f.gidByID[id], nil
}

type fakeGroupRepo struct {
	role group.Role
}

func (f fakeGroupRepo) RoleForUser(ctx context.Context, groupID, userID uuid.UUID) (group.Role, error) {
	return f.role, nil
}
func (f fakeGroupRepo) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]group.Group, error) {
	return nil, nil
}
func (f fakeGroupRepo) Create(ctx context.Context, slug, name string) (group.Group, error) {
	return group.Group{}, nil
}
func (f fakeGroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID, role group.Role) error {
	return nil
}
func (f fakeGroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]group.Member, error) {
	return nil, nil
}
func (f fakeGroupRepo) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error { return nil }
func (f fakeGroupRepo) FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f fakeGroupRepo) Count(ctx context.Context) (int, error) {
	return 0, nil
}
func (f fakeGroupRepo) GetByID(ctx context.Context, groupID uuid.UUID) (group.Group, error) {
	return group.Group{}, nil
}
func (f fakeGroupRepo) GetBySlug(ctx context.Context, slug string) (group.Group, error) {
	return group.Group{}, nil
}
func (f fakeGroupRepo) ListAll(ctx context.Context) ([]group.Group, error) {
	return nil, nil
}

func TestCreateRequiresWriterForNonAdmin(t *testing.T) {
	ctx := context.Background()
	pr := &fakeProductRepo{}
	gr := fakeGroupRepo{role: group.RoleReader}
	svc := New(pr, gr)

	cmd := CreateCommand{GroupID: uuid.New(), Name: "A", EntryDate: time.Now(), Status: "draft"}
	_, err := svc.Create(ctx, Actor{UserID: uuid.New(), IsAdmin: false}, cmd)
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateAllowsWriterForNonAdmin(t *testing.T) {
	ctx := context.Background()
	pr := &fakeProductRepo{}
	gr := fakeGroupRepo{role: group.RoleWriter}
	svc := New(pr, gr)

	cmd := CreateCommand{GroupID: uuid.New(), Name: "A", EntryDate: time.Now(), Status: "draft"}
	_, err := svc.Create(ctx, Actor{UserID: uuid.New(), IsAdmin: false}, cmd)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestGetByIDAllowsAdminWithoutRoleLookup(t *testing.T) {
	ctx := context.Background()
	pr := &fakeProductRepo{}
	gr := fakeGroupRepo{role: group.RoleReader}
	svc := New(pr, gr)

	gid := uuid.New()
	p, _ := product.New(gid, "A", "", time.Now(), nil, product.StatusDraft, false, 0, "")
	created, err := pr.Create(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.GetByID(ctx, Actor{UserID: uuid.New(), IsAdmin: true}, created.ID)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

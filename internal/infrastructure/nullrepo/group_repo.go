package nullrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
)

type GroupRepo struct{}

func NewGroupRepo() *GroupRepo { return &GroupRepo{} }

func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]group.Group, error) {
	return nil, errors.New("storage_not_configured")
}

func (r *GroupRepo) ListAll(ctx context.Context) ([]group.Group, error) {
	return nil, errors.New("storage_not_configured")
}

func (r *GroupRepo) Count(ctx context.Context) (int, error) {
	return 0, errors.New("storage_not_configured")
}

func (r *GroupRepo) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error) {
	return group.Group{}, errors.New("storage_not_configured")
}

func (r *GroupRepo) GetBySlug(ctx context.Context, slug string) (group.Group, error) {
	return group.Group{}, errors.New("storage_not_configured")
}

func (r *GroupRepo) Create(ctx context.Context, slug, name string) (group.Group, error) {
	return group.Group{}, errors.New("storage_not_configured")
}

func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID, role group.Role) error {
	return errors.New("storage_not_configured")
}

func (r *GroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]group.Member, error) {
	return nil, errors.New("storage_not_configured")
}

func (r *GroupRepo) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return errors.New("storage_not_configured")
}

func (r *GroupRepo) RoleForUser(ctx context.Context, userID, groupID uuid.UUID) (group.Role, error) {
	return group.Role(""), errors.New("storage_not_configured")
}

func (r *GroupRepo) FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	return uuid.Nil, errors.New("storage_not_configured")
}

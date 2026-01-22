package group

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]Group, error)
	ListAll(ctx context.Context) ([]Group, error)
	Count(ctx context.Context) (int, error)
	GetByID(ctx context.Context, id uuid.UUID) (Group, error)
	GetBySlug(ctx context.Context, slug string) (Group, error)
	Create(ctx context.Context, slug, name string) (Group, error)
	AddMember(ctx context.Context, groupID, userID uuid.UUID, role Role) error
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]Member, error)
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	RoleForUser(ctx context.Context, userID, groupID uuid.UUID) (Role, error)
	FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error)
}

type Member struct {
	UserID   uuid.UUID
	Username string
	Role     Role
}

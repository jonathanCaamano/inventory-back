package groups

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
)

type Service struct {
	repo group.Repository
}

func New(repo group.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]group.Group, error) {
	return s.repo.ListForUser(ctx, userID, isAdmin)
}

func (s *Service) ListAll(ctx context.Context) ([]group.Group, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) Create(ctx context.Context, slug, name string) (group.Group, error) {
	slug = strings.TrimSpace(slug)
	name = strings.TrimSpace(name)
	if slug == "" || name == "" {
		return group.Group{}, errors.New("slug_and_name_required")
	}
	return s.repo.Create(ctx, slug, name)
}

func (s *Service) AddMemberByUsername(ctx context.Context, groupID uuid.UUID, username string, role group.Role) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username_required")
	}
	if role != group.RoleReader && role != group.RoleWriter {
		return errors.New("invalid_role")
	}
	uid, err := s.repo.FindUserIDByUsername(ctx, username)
	if err != nil {
		return err
	}
	return s.repo.AddMember(ctx, groupID, uid, role)
}

func (s *Service) ListMembers(ctx context.Context, groupID uuid.UUID) ([]group.Member, error) {
	return s.repo.ListMembers(ctx, groupID)
}

func (s *Service) RemoveMemberByUsername(ctx context.Context, groupID uuid.UUID, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username_required")
	}
	uid, err := s.repo.FindUserIDByUsername(ctx, username)
	if err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, groupID, uid)
}

func (s *Service) RoleForUser(ctx context.Context, userID, groupID uuid.UUID) (group.Role, error) {
	return s.repo.RoleForUser(ctx, userID, groupID)
}

package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
	"github.com/jonathanCaamano/inventory-back/internal/domain/user"
)

type Service struct {
	users     user.Repository
	groups    group.Repository
	jwtSecret string
	jwtTTL    time.Duration
}

func New(users user.Repository, groups group.Repository, jwtSecret string, jwtTTL time.Duration) *Service {
	return &Service{users: users, groups: groups, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", errors.New("invalid_credentials")
	}

	uid, hash, isAdmin, isActive, err := s.users.GetForLogin(ctx, username)
	if err != nil {
		return "", errors.New("invalid_credentials")
	}
	if !isActive {
		return "", errors.New("user_inactive")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", errors.New("invalid_credentials")
	}

	return s.sign(uid, isAdmin)
}

type RegisterResult struct {
	UserID   uuid.UUID
	Username string
	Group    group.Group
	Role     group.Role
	Token    string
}

func (s *Service) Register(ctx context.Context, username, password string, groupID uuid.UUID, groupSlug, groupName string) (RegisterResult, error) {
	username = strings.TrimSpace(username)
	groupSlug = strings.TrimSpace(groupSlug)
	groupName = strings.TrimSpace(groupName)

	if username == "" || len(username) < 3 {
		return RegisterResult{}, errors.New("invalid_username")
	}
	if len(password) < 8 {
		return RegisterResult{}, errors.New("invalid_password")
	}

	g, created, err := s.resolveGroup(ctx, groupID, groupSlug, groupName)
	if err != nil {
		return RegisterResult{}, err
	}

	uid, err := s.users.Create(ctx, username, password, false)
	if err != nil {
		if err.Error() == "username_taken" {
			return RegisterResult{}, errors.New("username_taken")
		}
		return RegisterResult{}, err
	}

	role := group.RoleReader
	if created {
		role = group.RoleWriter
	}
	if err := s.groups.AddMember(ctx, g.ID, uid, role); err != nil {
		return RegisterResult{}, err
	}

	tok, err := s.sign(uid, false)
	if err != nil {
		return RegisterResult{}, err
	}

	return RegisterResult{UserID: uid, Username: username, Group: g, Role: role, Token: tok}, nil
}

func (s *Service) sign(uid uuid.UUID, isAdmin bool) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   uid.String(),
		"admin": isAdmin,
		"iat":   now.Unix(),
		"exp":   now.Add(s.jwtTTL).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", errors.New("token_error")
	}
	return signed, nil
}

func (s *Service) resolveGroup(ctx context.Context, gid uuid.UUID, slug, name string) (group.Group, bool, error) {
	if gid != uuid.Nil {
		g, err := s.groups.GetByID(ctx, gid)
		if err != nil {
			return group.Group{}, false, errors.New("group_not_found")
		}
		return g, false, nil
	}

	// group is mandatory: either an existing group_id or a new/existing slug/name must be provided.
	if slug == "" && name == "" {
		return group.Group{}, false, errors.New("group_required")
	}

	if slug != "" {
		g, err := s.groups.GetBySlug(ctx, slug)
		if err == nil {
			return g, false, nil
		}
		if name == "" {
			name = slug
		}
		ng, err := s.groups.Create(ctx, slug, name)
		if err != nil {
			return group.Group{}, false, err
		}
		return ng, true, nil
	}

	if name != "" {
		slug = slugify(name)
		if slug == "" {
			return group.Group{}, false, errors.New("invalid_group")
		}
		g, err := s.groups.GetBySlug(ctx, slug)
		if err == nil {
			return g, false, nil
		}
		ng, err := s.groups.Create(ctx, slug, name)
		if err != nil {
			return group.Group{}, false, err
		}
		return ng, true, nil
	}

	return group.Group{}, false, errors.New("group_required")
}

var nonSlug = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "-")
	v = nonSlug.ReplaceAllString(v, "")
	v = strings.Trim(v, "-")
	for strings.Contains(v, "--") {
		v = strings.ReplaceAll(v, "--", "-")
	}
	return v
}

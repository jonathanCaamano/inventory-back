package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
)

type GroupRepo struct {
	pool *pgxpool.Pool
}

func NewGroupRepo(pool *pgxpool.Pool) *GroupRepo {
	return &GroupRepo{pool: pool}
}

func (r *GroupRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM groups`).Scan(&n)
	return n, err
}

func (r *GroupRepo) ListAll(ctx context.Context) ([]group.Group, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, slug, name FROM groups ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []group.Group{}
	for rows.Next() {
		var g group.Group
		if err := rows.Scan(&g.ID, &g.Slug, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *GroupRepo) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error) {
	var g group.Group
	err := r.pool.QueryRow(ctx, `SELECT id, slug, name FROM groups WHERE id=$1`, id).Scan(&g.ID, &g.Slug, &g.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return group.Group{}, errors.New("not_found")
		}
		return group.Group{}, err
	}
	return g, nil
}

func (r *GroupRepo) GetBySlug(ctx context.Context, slug string) (group.Group, error) {
	var g group.Group
	err := r.pool.QueryRow(ctx, `SELECT id, slug, name FROM groups WHERE slug=$1`, slug).Scan(&g.ID, &g.Slug, &g.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return group.Group{}, errors.New("not_found")
		}
		return group.Group{}, err
	}
	return g, nil
}

func (r *GroupRepo) Create(ctx context.Context, slug, name string) (group.Group, error) {
	var g group.Group
	err := r.pool.QueryRow(ctx, `INSERT INTO groups (slug, name) VALUES ($1,$2) RETURNING id, slug, name`, slug, name).Scan(&g.ID, &g.Slug, &g.Name)
	return g, err
}

func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]group.Group, error) {
	rows, err := func() (pgx.Rows, error) {
		if isAdmin {
			return r.pool.Query(ctx, `SELECT id, slug, name FROM groups ORDER BY name ASC`)
		}
		return r.pool.Query(ctx, `
			SELECT g.id, g.slug, g.name
			FROM groups g
			JOIN group_memberships gm ON gm.group_id=g.id
			WHERE gm.user_id=$1
			ORDER BY g.name ASC`, userID)
	}()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []group.Group{}
	for rows.Next() {
		var g group.Group
		if err := rows.Scan(&g.ID, &g.Slug, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID, role group.Role) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO group_memberships (user_id, group_id, role) VALUES ($1,$2,$3)
		ON CONFLICT (user_id, group_id) DO UPDATE SET role=EXCLUDED.role`, userID, groupID, string(role))
	return err
}

func (r *GroupRepo) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM group_memberships WHERE user_id=$1 AND group_id=$2`, userID, groupID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("not_found")
	}
	return nil
}

func (r *GroupRepo) RoleForUser(ctx context.Context, userID, groupID uuid.UUID) (group.Role, error) {
	var role string
	err := r.pool.QueryRow(ctx, `SELECT role FROM group_memberships WHERE user_id=$1 AND group_id=$2`, userID, groupID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("not_member")
		}
		return "", err
	}
	role = strings.TrimSpace(role)
	if role == string(group.RoleReader) {
		return group.RoleReader, nil
	}
	if role == string(group.RoleWriter) {
		return group.RoleWriter, nil
	}
	return "", errors.New("invalid_role")
}

func (r *GroupRepo) FindUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT id FROM users WHERE username=$1`, username).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errors.New("not_found")
		}
		return uuid.Nil, err
	}
	return id, nil
}

type memberRow struct {
	UserID    uuid.UUID
	Role      string
	Username  string
	CreatedAt time.Time
}

func (r *GroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]group.Member, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT gm.user_id, gm.role, u.username, gm.created_at
		FROM group_memberships gm
		JOIN users u ON u.id=gm.user_id
		WHERE gm.group_id=$1
		ORDER BY u.username ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []group.Member{}
	for rows.Next() {
		var m memberRow
		if err := rows.Scan(&m.UserID, &m.Role, &m.Username, &m.CreatedAt); err != nil {
			return nil, err
		}
		rr := group.Role(strings.TrimSpace(m.Role))
		out = append(out, group.Member{UserID: m.UserID, Username: m.Username, Role: rr})
	}
	return out, nil
}

var _ group.Repository = (*GroupRepo)(nil)

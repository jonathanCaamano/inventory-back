package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/jonathanCaamano/inventory-back/internal/domain/user"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) EnsureBootstrapAdmin(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE is_admin=true)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO users (username, password_hash, is_admin, is_active) VALUES ($1,$2,true,true)`, username, string(hash))
	return err
}

func (r *UserRepo) GetForLogin(ctx context.Context, username string) (id uuid.UUID, passwordHash string, isAdmin bool, isActive bool, err error) {
	err = r.pool.QueryRow(ctx, `SELECT id, password_hash, is_admin, is_active FROM users WHERE username=$1`, username).Scan(&id, &passwordHash, &isAdmin, &isActive)
	return
}

func (r *UserRepo) Create(ctx context.Context, username, password string, isAdmin bool) (uuid.UUID, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = r.pool.QueryRow(ctx, `INSERT INTO users (username, password_hash, is_admin, is_active) VALUES ($1,$2,$3,true) RETURNING id`, username, string(hash), isAdmin).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, errors.New("username_taken")
		}
		return uuid.Nil, err
	}
	return id, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	var u user.User
	err := r.pool.QueryRow(ctx, `SELECT id, username, is_admin, is_active FROM users WHERE id=$1`, id).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.IsActive)
	return u, err
}

func (r *UserRepo) List(ctx context.Context, search string, limit, offset int) ([]user.User, int, error) {
	args := []any{}
	where := ""
	if search != "" {
		where = "WHERE username ILIKE $1"
		args = append(args, "%"+search+"%")
	}
	var total int
	countQ := `SELECT count(*) FROM users ` + where
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args2 := append([]any{}, args...)
	args2 = append(args2, limit, offset)
	q := `SELECT id, username, is_admin, is_active FROM users ` + where + ` ORDER BY username ASC LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	rows, err := r.pool.Query(ctx, q, args2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []user.User{}
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.IsActive); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, nil
}

func itoa(i int) string {
	// small helper to avoid strconv import churn across files
	buf := [16]byte{}
	n := len(buf)
	v := i
	if v == 0 {
		return "0"
	}
	for v > 0 {
		n--
		buf[n] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[n:])
}

var _ user.Repository = (*UserRepo)(nil)

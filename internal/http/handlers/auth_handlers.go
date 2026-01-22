package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/application/auth"
	"github.com/jonathanCaamano/inventory-back/internal/http/response"
)

type Auth struct {
	svc *auth.Service
}

func NewAuth(svc *auth.Service) *Auth {
	return &Auth{svc: svc}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerReq struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	GroupID   uuid.UUID `json:"group_id"`
	GroupSlug string    `json:"group_slug"`
	GroupName string    `json:"group_name"`
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, 400, "invalid_json")
		return
	}

	tok, err := a.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		code := 401
		if err.Error() == "user_inactive" {
			code = 403
		}
		response.Error(w, code, err.Error())
		return
	}

	response.JSON(w, 200, map[string]any{"access_token": tok, "token_type": "Bearer"})
}

func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, 400, "invalid_json")
		return
	}

	out, err := a.svc.Register(r.Context(), in.Username, in.Password, in.GroupID, in.GroupSlug, in.GroupName)
	if err != nil {
		switch err.Error() {
		case "invalid_username", "invalid_password", "group_required", "invalid_group":
			response.Error(w, 400, err.Error())
			return
		case "group_not_found":
			response.Error(w, 404, "group_not_found")
			return
		case "username_taken":
			response.Error(w, 409, "username_taken")
			return
		default:
			response.Error(w, 500, "internal_error")
			return
		}
	}

	response.JSON(w, 201, map[string]any{
		"access_token": out.Token,
		"token_type":   "Bearer",
		"user":         map[string]any{"id": out.UserID, "username": out.Username, "is_admin": false},
		"group":        out.Group,
		"role":         string(out.Role),
	})
}

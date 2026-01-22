package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jonathanCaamano/inventory-back/internal/application/groups"
	"github.com/jonathanCaamano/inventory-back/internal/domain/group"
	"github.com/jonathanCaamano/inventory-back/internal/http/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/http/response"
)

type Groups struct {
	svc *groups.Service
}

func NewGroups(svc *groups.Service) *Groups {
	return &Groups{svc: svc}
}

func (h *Groups) PublicList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListAll(r.Context())
	if err != nil {
		response.Error(w, 500, "internal_error")
		return
	}
	response.JSON(w, 200, map[string]any{"items": items})
}

func (h *Groups) ListForMe(w http.ResponseWriter, r *http.Request) {
	uid, err := middleware.UserID(r.Context())
	if err != nil {
		response.Error(w, 401, "missing_token")
		return
	}
	isAdmin := middleware.IsAdmin(r.Context())
	items, err := h.svc.ListForUser(r.Context(), uid, isAdmin)
	if err != nil {
		response.Error(w, 500, "internal_error")
		return
	}
	response.JSON(w, 200, map[string]any{"items": items})
}

type adminCreateGroupReq struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (h *Groups) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var in adminCreateGroupReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, 400, "invalid_json")
		return
	}
	g, err := h.svc.Create(r.Context(), in.Slug, in.Name)
	if err != nil {
		response.Error(w, 400, err.Error())
		return
	}
	response.JSON(w, 201, g)
}

type adminMemberReq struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (h *Groups) AdminAddMember(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, 400, "invalid_group_id")
		return
	}
	var in adminMemberReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, 400, "invalid_json")
		return
	}
	role := group.Role(in.Role)
	if err := h.svc.AddMemberByUsername(r.Context(), gid, in.Username, role); err != nil {
		if err.Error() == "not_found" {
			response.Error(w, 404, "user_not_found")
			return
		}
		response.Error(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (h *Groups) AdminListMembers(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, 400, "invalid_group_id")
		return
	}
	items, err := h.svc.ListMembers(r.Context(), gid)
	if err != nil {
		response.Error(w, 500, "internal_error")
		return
	}
	response.JSON(w, 200, map[string]any{"items": items})
}

func (h *Groups) AdminRemoveMember(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, 400, "invalid_group_id")
		return
	}
	username := r.URL.Query().Get("username")
	if err := h.svc.RemoveMemberByUsername(r.Context(), gid, username); err != nil {
		if err.Error() == "not_found" {
			response.Error(w, 404, "user_not_found")
			return
		}
		response.Error(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}

package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"example.com/go-templ-react-admin/internal/auth"
	"example.com/go-templ-react-admin/internal/views"
)

type handlers struct {
	dev     bool
	logger  *log.Logger
	store   *auth.Store
	session *auth.SessionManager
}

func (h *handlers) currentUser(r *http.Request) (*auth.User, bool) {
	u, ok := r.Context().Value(ctxKeyUser).(*auth.User)
	return u, ok && u != nil
}

func (h *handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := h.session.Get(r)
		if err == nil {
			if raw, ok := sess.Values["user_id"]; ok {
				switch v := raw.(type) {
				case int64:
					if u, err := h.store.GetUserByID(v); err == nil {
						ctx := context.WithValue(r.Context(), ctxKeyUser, u)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				case int:
					if u, err := h.store.GetUserByID(int64(v)); err == nil {
						ctx := context.WithValue(r.Context(), ctxKeyUser, u)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				case string:
					if id, err := strconv.ParseInt(v, 10, 64); err == nil {
						if u, err := h.store.GetUserByID(id); err == nil {
							ctx := context.WithValue(r.Context(), ctxKeyUser, u)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}
		}

		if wantsHTML(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
	})
}

func (h *handlers) getLogin(w http.ResponseWriter, r *http.Request) {
	views.LoginPage(views.LoginPageProps{
		Dev: h.dev,
		Error: "",
	}).Render(r.Context(), w)
}

func (h *handlers) postLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		views.LoginPage(views.LoginPageProps{Dev: h.dev, Error: "入力が不正です"}).Render(r.Context(), w)
		return
	}
	email := strings.TrimSpace(r.Form.Get("email"))
	pass := r.Form.Get("password")

	u, err := h.store.Authenticate(email, pass)
	if err != nil {
		views.LoginPage(views.LoginPageProps{Dev: h.dev, Error: "メールアドレスまたはパスワードが違います"}).Render(r.Context(), w)
		return
	}

	sess, _ := h.session.Get(r)
	sess.Values["user_id"] = u.ID
	_ = h.session.Save(r, w, sess)
	_ = h.store.WriteAudit(&u.ID, "login", "user", strconv.FormatInt(u.ID, 10), map[string]any{"email": u.Email})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handlers) postLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := h.session.Get(r)
	delete(sess.Values, "user_id")
	sess.Options.MaxAge = -1
	_ = h.session.Save(r, w, sess)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *handlers) getDashboard(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	views.DashboardPage(views.AuthLayoutProps{
		Dev:  h.dev,
		User: u,
		Title: "Dashboard",
	}, views.DashboardProps{}).Render(r.Context(), w)
}

func (h *handlers) getUsersPage(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	views.UsersPage(views.AuthLayoutProps{
		Dev:  h.dev,
		User: u,
		Title: "Users",
	}, views.UsersProps{
	}).Render(r.Context(), w)
}

func (h *handlers) getProjectsPage(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	views.ProjectsPage(views.AuthLayoutProps{
		Dev:  h.dev,
		User: u,
		Title: "Projects",
	}, views.ProjectsProps{}).Render(r.Context(), w)
}

func (h *handlers) apiMe(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role, "isActive": u.IsActive,
	})
}

func (h *handlers) apiListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	role := auth.Role(r.URL.Query().Get("role"))
	offset, limit := parsePage(r)
	users, total, err := h.store.ListUsers(q, role, offset, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users, "total": total, "offset": offset, "limit": limit})
}

func (h *handlers) apiUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	if u.Role != auth.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var body struct {
		Role auth.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	updated, err := h.store.UpdateUserRole(id, body.Role)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	_ = h.store.WriteAudit(&u.ID, "update_role", "user", strconv.FormatInt(id, 10), map[string]any{"role": body.Role})
	writeJSON(w, http.StatusOK, updated)
}

func (h *handlers) apiSetUserActive(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	if u.Role != auth.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	updated, err := h.store.SetUserActive(id, body.Active)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	_ = h.store.WriteAudit(&u.ID, "set_active", "user", strconv.FormatInt(id, 10), map[string]any{"active": body.Active})
	writeJSON(w, http.StatusOK, updated)
}

func (h *handlers) apiResetUserPassword(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	if u.Role != auth.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	if err := h.store.ResetUserPassword(id, body.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	_ = h.store.WriteAudit(&u.ID, "reset_password", "user", strconv.FormatInt(id, 10), nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) apiListProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	offset, limit := parsePage(r)
	items, total, err := h.store.ListProjects(q, status, offset, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "offset": offset, "limit": limit})
}

func (h *handlers) apiCreateProject(w http.ResponseWriter, r *http.Request) {
	u, _ := h.currentUser(r)
	if u.Role != auth.RoleAdmin && u.Role != auth.RoleEditor {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	var body auth.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	p, err := h.store.CreateProject(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	_ = h.store.WriteAudit(&u.ID, "create_project", "project", strconv.FormatInt(p.ID, 10), map[string]any{"key": p.Key})
	writeJSON(w, http.StatusCreated, p)
}

func parsePage(r *http.Request) (offset, limit int) {
	limit = 50
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *handlers) badRequest(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "bad request"
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": msg})
}

var errForbidden = errors.New("forbidden")


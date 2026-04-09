package httpx

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"example.com/go-templ-react-admin/internal/auth"
	"example.com/go-templ-react-admin/internal/views"
)

type RouterDeps struct {
	Dev     bool
	Logger  *log.Logger
	DB      *sql.DB
	Store   *auth.Store
	Session *auth.SessionManager
}

type ctxKey string

const ctxKeyUser ctxKey = "user"

func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(withLogger(d.Logger))

	// Static assets (prod build output).
	r.Mount("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("static/assets"))))

	// Health.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	h := &handlers{
		dev:     d.Dev,
		logger:  d.Logger,
		store:   d.Store,
		session: d.Session,
	}

	r.Get("/login", h.getLogin)
	r.Post("/login", h.postLogin)
	r.Post("/logout", h.postLogout)

	r.Group(func(protected chi.Router) {
		protected.Use(h.requireAuth)

		protected.Get("/", h.getDashboard)
		protected.Get("/users", h.getUsersPage)
		protected.Get("/projects", h.getProjectsPage)

		protected.Route("/api", func(api chi.Router) {
			api.Get("/me", h.apiMe)
			api.Get("/users", h.apiListUsers)
			api.Post("/users/{id}/role", h.apiUpdateUserRole)
			api.Post("/users/{id}/active", h.apiSetUserActive)
			api.Post("/users/{id}/reset-password", h.apiResetUserPassword)

			api.Get("/projects", h.apiListProjects)
			api.Post("/projects", h.apiCreateProject)
		})
	})

	// 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		views.ErrorPage("Not Found", "ページが見つかりません").Render(r.Context(), w)
	})

	return r
}

func withLogger(l *log.Logger) func(http.Handler) http.Handler {
	if l == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			l.Printf("%s %s %d %s", r.Method, r.URL.Path, ww.Status(), time.Since(start).Truncate(time.Millisecond))
		})
	}
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return true
	}
	return strings.Contains(accept, "text/html")
}


package app

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"example.com/go-templ-react-admin/internal/auth"
	"example.com/go-templ-react-admin/internal/db"
	"example.com/go-templ-react-admin/internal/httpx"
)

type Config struct {
	Addr      string
	DBPath    string
	Dev       bool
	SeedAdmin bool
	CookieKey string
	Logger    *log.Logger
}

type Server struct {
	cfg    Config
	logger *log.Logger
	db     *sql.DB
	router http.Handler
	closer func() error
}

func New(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "data/app.db"
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if cfg.CookieKey == "" {
		return nil, errors.New("CookieKey required")
	}
	if _, err := base64.StdEncoding.DecodeString(cfg.CookieKey); err != nil {
		return nil, errors.New("cookie key must be base64")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	database.SetConnMaxLifetime(0)

	if err := db.Migrate(database); err != nil {
		_ = database.Close()
		return nil, err
	}

	store := auth.NewStore(database)
	if cfg.SeedAdmin {
		if err := store.SeedDefaultAdmin(); err != nil {
			_ = database.Close()
			return nil, err
		}
	}

	session, err := auth.NewSessionManager(cfg.CookieKey, cfg.Dev)
	if err != nil {
		_ = database.Close()
		return nil, err
	}

	r := httpx.NewRouter(httpx.RouterDeps{
		Dev:     cfg.Dev,
		Logger:  cfg.Logger,
		DB:      database,
		Store:   store,
		Session: session,
	})

	return &Server{
		cfg:    cfg,
		logger: cfg.Logger,
		db:     database,
		router: r,
		closer: func() error {
			_ = session.Close()
			// Ensure any pending SQLite work is flushed.
			database.SetConnMaxLifetime(1 * time.Nanosecond)
			return database.Close()
		},
	}, nil
}

func (s *Server) Addr() string       { return s.cfg.Addr }
func (s *Server) Handler() http.Handler { return s.router }
func (s *Server) DB() *sql.DB        { return s.db }

func (s *Server) Close() error {
	if s.closer == nil {
		return nil
	}
	return s.closer()
}


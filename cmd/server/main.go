package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/go-templ-react-admin/internal/app"
)

func main() {
	var (
		addr  = flag.String("addr", "127.0.0.1:8080", "listen address")
		db    = flag.String("db", "data/app.db", "sqlite db path")
		dev   = flag.Bool("dev", true, "dev mode (less caching, extra logs)")
		seed  = flag.Bool("seed", true, "seed default admin user if empty")
		ckKey = flag.String("cookie-key", "", "base64 cookie auth key (optional)")
	)
	flag.Parse()

	cookieKey := *ckKey
	if cookieKey == "" {
		// Dev-friendly default: generate a random key on each boot.
		// In production, pass -cookie-key and keep it stable.
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("rand: %v", err)
		}
		cookieKey = base64.StdEncoding.EncodeToString(b)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	server, err := app.New(app.Config{
		Addr:      *addr,
		DBPath:    *db,
		Dev:       *dev,
		SeedAdmin: *seed,
		CookieKey: cookieKey,
		Logger:    logger,
	})
	if err != nil {
		logger.Fatalf("init: %v", err)
	}

	httpServer := &http.Server{
		Addr:         server.Addr(),
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Printf("listening on http://%s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	_ = server.Close()
	logger.Printf("shutdown complete")
}


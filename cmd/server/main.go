package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	maxUploadSize := int64(50 * 1024 * 1024) // 50MB default
	if v := os.Getenv("MAX_UPLOAD_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			maxUploadSize = n
		}
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}

	mux := http.NewServeMux()

	h := &Handler{MaxUploadSize: maxUploadSize}
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /api/generate", h.Generate)
	mux.HandleFunc("POST /api/template/parse", h.ParseTemplate)

	// Serve frontend static files if the directory exists
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		slog.Info("Serving static files", "dir", staticDir)
		mux.Handle("/", StaticHandler(staticDir))
	}

	handler := LoggingMiddleware(CORSMiddleware(mux, allowedOrigins))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped")
}

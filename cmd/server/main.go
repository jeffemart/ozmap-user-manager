package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/example/ozmap-user-manager/internal/auth"
	"github.com/example/ozmap-user-manager/internal/config"
	"github.com/example/ozmap-user-manager/internal/handlers"
	"github.com/example/ozmap-user-manager/internal/middleware"
	"github.com/example/ozmap-user-manager/internal/ozmap"
	"github.com/example/ozmap-user-manager/internal/repositories"
	"github.com/example/ozmap-user-manager/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database pool creation failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	if err := repositories.Migrate(ctx, db); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	roleRepo := repositories.NewRoleRepository(db)
	groupRepo := repositories.NewProjectGroupRepository(db)
	importRepo := repositories.NewImportRepository(db)
	ozClient := ozmap.NewClient(cfg)
	roleService := services.NewRoleService(roleRepo)
	groupService := services.NewProjectGroupService(groupRepo)
	importService := services.NewImportService(importRepo, roleRepo, groupService, ozClient, cfg)
	authService := auth.NewService(cfg.AdminUsername, cfg.AdminPasswordHash, cfg.SessionSecret)
	sessionStore := auth.NewSessionStore(cfg.SessionSecret)

	tmpl, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		panic(err)
	}
	h := handlers.New(tmpl, staticFiles, roleService, groupService, importService, authService, sessionStore, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /logout", h.Logout)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSub(staticFiles, "static")))))
	mux.HandleFunc("GET /download/template", h.DownloadTemplate)
	mux.Handle("/", middleware.RequireAuth(sessionStore, http.HandlerFunc(h.Dashboard)))
	mux.Handle("/roles", middleware.RequireAuth(sessionStore, http.HandlerFunc(h.RolesPage)))
	mux.Handle("/project-groups", middleware.RequireAuth(sessionStore, http.HandlerFunc(h.ProjectGroupsPage)))
	mux.Handle("/import", middleware.RequireAuth(sessionStore, http.HandlerFunc(h.ImportPage)))
	mux.Handle("/history", middleware.RequireAuth(sessionStore, http.HandlerFunc(h.HistoryPage)))
	mux.Handle("/api/roles", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.RolesAPI))))
	mux.Handle("/api/project-groups", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.ProjectGroupsAPI))))
	mux.Handle("/api/upload", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.UploadAPI))))
	mux.Handle("/api/create", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.CreateAPI))))
	mux.Handle("GET /api/create/{id}", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.CreationProgressAPI))))
	mux.Handle("/api/history", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.HistoryListAPI))))
	mux.Handle("/api/history/", middleware.RequireAuth(sessionStore, middleware.CSRF(sessionStore, http.HandlerFunc(h.HistoryDetailAPI))))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: middleware.SecurityHeaders(middleware.RateLimit(mux, cfg)), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 120 * time.Second}
	logger.Info("server started", "port", cfg.Port, "env", cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func mustSub(f embed.FS, dir string) fs.FS {
	x, err := fs.Sub(f, dir)
	if err != nil {
		panic(fmt.Sprintf("static fs: %v", err))
	}
	return x
}

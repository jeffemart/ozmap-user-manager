package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv, Port, DatabaseURL, OZMapBaseURL, OZMapClientID, OZMapClientSecret, OZMapAuthorization, AdminUsername, AdminPasswordHash, SessionSecret, PasswordMode string
	MaxUploadSizeMB, MaxImportRows                                                                                                                               int
	LogLevel                                                                                                                                                     slog.Level
	OZMapTimeout                                                                                                                                                 time.Duration
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func Load() (Config, error) {
	c := Config{AppEnv: env("APP_ENV", "development"), Port: env("APP_PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"), OZMapBaseURL: os.Getenv("OZMAP_BASE_URL"), OZMapClientID: os.Getenv("OZMAP_CLIENT_ID"), OZMapClientSecret: os.Getenv("OZMAP_CLIENT_SECRET"), OZMapAuthorization: os.Getenv("OZMAP_AUTHORIZATION"), AdminUsername: os.Getenv("ADMIN_USERNAME"), AdminPasswordHash: os.Getenv("ADMIN_PASSWORD_HASH"), SessionSecret: os.Getenv("SESSION_SECRET"), PasswordMode: env("OZMAP_PASSWORD_MODE", "plain"), LogLevel: slog.LevelInfo}
	c.MaxUploadSizeMB = atoi(env("MAX_UPLOAD_SIZE_MB", "5"), 5)
	c.MaxImportRows = atoi(env("MAX_IMPORT_ROWS", "1000"), 1000)
	c.OZMapTimeout = time.Duration(atoi(env("OZMAP_HTTP_TIMEOUT_SECONDS", "30"), 30)) * time.Second
	if c.DatabaseURL == "" || c.OZMapBaseURL == "" || c.AdminUsername == "" || c.AdminPasswordHash == "" || len(c.SessionSecret) < 32 {
		return c, fmt.Errorf("required configuration is missing (DATABASE_URL, OZMAP_BASE_URL, ADMIN_USERNAME, ADMIN_PASSWORD_HASH, SESSION_SECRET)")
	}
	return c, nil
}
func atoi(v string, d int) int {
	x, e := strconv.Atoi(v)
	if e != nil || x <= 0 {
		return d
	}
	return x
}

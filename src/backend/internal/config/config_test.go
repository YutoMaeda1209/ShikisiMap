package config

import (
	"strings"
	"testing"
)

var envKeys = []string{
	"LISTEN_ADDR",
	"DATABASE_URL",
	"PUBLIC_ORIGIN",
	"ADMIN_OIDC_AUDIENCE",
	"GOOGLE_OAUTH_CLIENT_ID",
	"GOOGLE_OAUTH_CLIENT_SECRET",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/shikisimap")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.PublicOrigin != "http://localhost:5173" {
		t.Errorf("PublicOrigin = %q", cfg.PublicOrigin)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	clearEnv(t)
	t.Setenv("LISTEN_ADDR", "127.0.0.1:8080")
	t.Setenv("DATABASE_URL", "postgres://localhost/shikisimap")
	t.Setenv("PUBLIC_ORIGIN", "https://map.mendako.dev")
	t.Setenv("ADMIN_OIDC_AUDIENCE", "https://map.mendako.dev/api/admin")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "client-secret")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		ListenAddr:              "127.0.0.1:8080",
		DatabaseURL:             "postgres://localhost/shikisimap",
		PublicOrigin:            "https://map.mendako.dev",
		AdminOIDCAudience:       "https://map.mendako.dev/api/admin",
		GoogleOAuthClientID:     "client-id",
		GoogleOAuthClientSecret: "client-secret",
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	clearEnv(t)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("err = %v, want an error naming DATABASE_URL", err)
	}
}

func TestLoadRejectsInvalidPublicOrigin(t *testing.T) {
	for _, origin := range []string{
		"map.mendako.dev",
		"https://map.mendako.dev/",
		"https://map.mendako.dev/app",
		"https://map.mendako.dev?x=1",
		"ftp://map.mendako.dev",
		"https://",
	} {
		t.Run(origin, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("DATABASE_URL", "postgres://localhost/shikisimap")
			t.Setenv("PUBLIC_ORIGIN", origin)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "PUBLIC_ORIGIN") {
				t.Errorf("err = %v, want an error naming PUBLIC_ORIGIN", err)
			}
		})
	}
}

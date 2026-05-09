package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_ReadsYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(`
server:
  addr: ":9090"
db:
  dsn: "host=127.0.0.1 port=5432 user=blog dbname=blog sslmode=disable"
redis:
  addr: "127.0.0.1:6379"
session:
  cookie_name: "sid"
  ttl_minutes: 30
  cookie_secret: "x"
upload:
  max_bytes: 1024
  allowed_ext: ["png"]
ratelimit:
  login_per_min: 5
  upload_per_min: 10
  global_per_min: 300
view_flush:
  interval_seconds: 30
  batch_size: 1000
log:
  level: "debug"
`), 0o644))

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.Server.Addr)
	require.Equal(t, 30, cfg.Session.TTLMinutes)
	require.Equal(t, []string{"png"}, cfg.Upload.AllowedExt)
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(`
server: { addr: ":9090" }
db:      { dsn: "from-yaml" }
redis:   { addr: "127.0.0.1:6379" }
session: { cookie_name: "sid", ttl_minutes: 30, cookie_secret: "x" }
upload:  { max_bytes: 1, allowed_ext: ["png"] }
ratelimit: { login_per_min: 5, upload_per_min: 10, global_per_min: 300 }
view_flush: { interval_seconds: 30, batch_size: 1000 }
log: { level: "info" }
`), 0o644))
	t.Setenv("DB_DSN", "from-env")

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	require.Equal(t, "from-env", cfg.DB.DSN)
}

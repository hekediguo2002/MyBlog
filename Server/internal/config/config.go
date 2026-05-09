package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerCfg    `mapstructure:"server"`
	DB        DBCfg        `mapstructure:"db"`
	Redis     RedisCfg     `mapstructure:"redis"`
	Session   SessionCfg   `mapstructure:"session"`
	Upload    UploadCfg    `mapstructure:"upload"`
	RateLimit RateLimitCfg `mapstructure:"ratelimit"`
	ViewFlush ViewFlushCfg `mapstructure:"view_flush"`
	Log       LogCfg       `mapstructure:"log"`
}

type ServerCfg struct {
	Addr             string `mapstructure:"addr"`
	StaticDir        string `mapstructure:"static_dir"`
	UploadDir        string `mapstructure:"upload_dir"`
	CSRFCookieSecure bool   `mapstructure:"csrf_cookie_secure"`
}
type DBCfg struct{ DSN string `mapstructure:"dsn"` }
type RedisCfg struct {
	Addr string `mapstructure:"addr"`
	DB   int    `mapstructure:"db"`
}
type SessionCfg struct {
	CookieName   string `mapstructure:"cookie_name"`
	TTLMinutes   int    `mapstructure:"ttl_minutes"`
	CookieSecret string `mapstructure:"cookie_secret"`
}
type UploadCfg struct {
	MaxBytes   int64    `mapstructure:"max_bytes"`
	AllowedExt []string `mapstructure:"allowed_ext"`
}
type RateLimitCfg struct {
	LoginPerMin  int `mapstructure:"login_per_min"`
	UploadPerMin int `mapstructure:"upload_per_min"`
	GlobalPerMin int `mapstructure:"global_per_min"`
}
type ViewFlushCfg struct {
	IntervalSeconds int `mapstructure:"interval_seconds"`
	BatchSize       int `mapstructure:"batch_size"`
}
type LogCfg struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bind := []string{
		"server.addr", "server.static_dir", "server.upload_dir", "server.csrf_cookie_secure",
		"db.dsn", "redis.addr", "redis.db",
		"session.cookie_name", "session.ttl_minutes", "session.cookie_secret",
		"log.level", "log.file",
	}
	for _, k := range bind {
		_ = v.BindEnv(k)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &cfg, nil
}

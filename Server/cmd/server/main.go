package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/wjr/blog/server/internal/cache"
	"github.com/wjr/blog/server/internal/config"
	"github.com/wjr/blog/server/internal/db"
	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/router"
	"github.com/wjr/blog/server/internal/service"
	"github.com/wjr/blog/server/internal/worker"
)

func extToMIME(exts []string) []string {
	mimes := make([]string, 0, len(exts))
	for _, e := range exts {
		switch e {
		case "jpg", "jpeg":
			mimes = append(mimes, "image/jpeg")
		case "png":
			mimes = append(mimes, "image/png")
		case "gif":
			mimes = append(mimes, "image/gif")
		case "webp":
			mimes = append(mimes, "image/webp")
		}
	}
	return mimes
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	cfgPath := os.Getenv("BLOG_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	gdb, err := db.Open(db.DefaultOptions(cfg.MySQL.DSN))
	if err != nil {
		log.Fatal().Err(err).Msg("open mysql")
	}

	rdb, err := cache.Open(cache.Default(cfg.Redis.Addr, cfg.Redis.DB))
	if err != nil {
		log.Fatal().Err(err).Msg("open redis")
	}

	userRepo := repository.NewUserRepo(gdb)
	tagRepo := repository.NewTagRepo(gdb)
	articleRepo := repository.NewArticleRepo(gdb)
	counterRepo := repository.NewCounterRepo(rdb)

	authSvc := service.NewAuthService(userRepo)
	tagSvc := service.NewTagService(tagRepo)
	articleSvc := service.NewArticleService(articleRepo, tagRepo, userRepo, counterRepo)
	uploadSvc := service.NewUploadService(service.UploadOptions{
		Dir:         cfg.Server.UploadDir,
		MaxBytes:    cfg.Upload.MaxBytes,
		AllowedMIME: extToMIME(cfg.Upload.AllowedExt),
	})

	sessions := middleware.NewSessionStore(rdb, cfg.Session.TTLMinutes)

	authH := handler.NewAuthHandler(authSvc, sessions, cfg.Server.CSRFCookieSecure)
	articleH := handler.NewArticleHandler(articleSvc)
	tagH := handler.NewTagHandler(tagSvc)
	uploadH := handler.NewUploadHandler(uploadSvc)

	r := router.New(router.Deps{
		Auth:            authH,
		Article:         articleH,
		Tag:             tagH,
		Upload:          uploadH,
		Sessions:        sessions,
		RDB:             rdb,
		StaticWebDir:    cfg.Server.StaticDir,
		StaticUploadDir: cfg.Server.UploadDir,
		SecureCookies:   cfg.Server.CSRFCookieSecure,
		RateLimitIP: middleware.RateLimitOpts{
			Name: "global", Max: cfg.RateLimit.GlobalPerMin, Window: 60, KeyFn: middleware.IPKey,
		},
		RateLimitUser: middleware.RateLimitOpts{
			Name: "upload", Max: cfg.RateLimit.UploadPerMin, Window: 60, KeyFn: middleware.UserKey,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	flushInterval := time.Duration(cfg.ViewFlush.IntervalSeconds) * time.Second
	if flushInterval == 0 {
		flushInterval = 30 * time.Second
	}
	flush := worker.New(counterRepo, articleRepo, flushInterval)
	go func() { defer wg.Done(); flush.Run(ctx) }()

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Str("addr", cfg.Server.Addr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server crashed")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info().Msg("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	cancel()
	wg.Wait()
	log.Info().Msg("bye")
}

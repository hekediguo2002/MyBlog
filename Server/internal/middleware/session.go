package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/idgen"
)

type Session struct {
	UserID uint64 `json:"uid"`
	Name   string `json:"name"`
}

type SessionStore struct {
	rdb        *redis.Client
	ttlMinutes int
}

func NewSessionStore(rdb *redis.Client, ttlMinutes int) *SessionStore {
	if ttlMinutes <= 0 {
		ttlMinutes = 30
	}
	return &SessionStore{rdb: rdb, ttlMinutes: ttlMinutes}
}

func (s *SessionStore) ttl() time.Duration { return time.Duration(s.ttlMinutes) * time.Minute }

func sessionKey(sid string) string { return "sess:" + sid }
func csrfKey(sid string) string    { return "csrf:" + sid }

func (s *SessionStore) Create(ctx context.Context, sess Session) (sid, csrf string, err error) {
	sid = idgen.NewUUID()
	csrf = idgen.NewUUID()
	body, err := json.Marshal(sess)
	if err != nil {
		return "", "", apperr.Wrap(apperr.CodeUnknown, "marshal session", err)
	}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, sessionKey(sid), body, s.ttl())
	pipe.Set(ctx, csrfKey(sid), csrf, s.ttl())
	if _, err := pipe.Exec(ctx); err != nil {
		return "", "", apperr.Wrap(apperr.CodeRedisError, "session create", err)
	}
	return sid, csrf, nil
}

func (s *SessionStore) Get(ctx context.Context, sid string) (*Session, error) {
	body, err := s.rdb.Get(ctx, sessionKey(sid)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperr.New(apperr.CodeUnauthorized, "未登录")
		}
		return nil, apperr.Wrap(apperr.CodeRedisError, "session get", err)
	}
	var sess Session
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, apperr.Wrap(apperr.CodeUnknown, "session decode", err)
	}
	return &sess, nil
}

func (s *SessionStore) GetCSRF(ctx context.Context, sid string) (string, error) {
	v, err := s.rdb.Get(ctx, csrfKey(sid)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", apperr.New(apperr.CodeCSRFInvalid, "csrf token 缺失")
		}
		return "", apperr.Wrap(apperr.CodeRedisError, "csrf get", err)
	}
	return v, nil
}

func (s *SessionStore) Touch(ctx context.Context, sid string) error {
	pipe := s.rdb.Pipeline()
	pipe.Expire(ctx, sessionKey(sid), s.ttl())
	pipe.Expire(ctx, csrfKey(sid), s.ttl())
	if _, err := pipe.Exec(ctx); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "session touch", err)
	}
	return nil
}

func (s *SessionStore) Delete(ctx context.Context, sid string) error {
	if err := s.rdb.Del(ctx, sessionKey(sid), csrfKey(sid)).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "session delete", err)
	}
	return nil
}

func (s *SessionStore) TTLSeconds() int { return s.ttlMinutes * 60 }

const ctxKeySession = "blog.session"

func AttachSession(c *gin.Context, sess Session) { c.Set(ctxKeySession, sess) }
func SessionFromContext(c *gin.Context) (Session, bool) {
	v, ok := c.Get(ctxKeySession)
	if !ok {
		return Session{}, false
	}
	s, ok := v.(Session)
	return s, ok
}

package featureflags

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type cachedFlag struct {
	enabled   bool
	expiresAt time.Time
}

type Store struct {
	pool  *pgxpool.Pool
	ttl   time.Duration
	mu    sync.RWMutex
	cache map[string]cachedFlag
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, ttl: 5 * time.Second, cache: make(map[string]cachedFlag)}
}

func (s *Store) Enabled(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	s.mu.RLock()
	cached, ok := s.cache[key]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) {
		return cached.enabled, nil
	}

	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT enabled FROM platform_feature_flags WHERE key=$1`, key).Scan(&enabled)
	if err != nil {
		if ok {
			return cached.enabled, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return true, err
	}

	s.mu.Lock()
	s.cache[key] = cachedFlag{enabled: enabled, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
	return enabled, nil
}

func (s *Store) Invalidate(key string) {
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
}

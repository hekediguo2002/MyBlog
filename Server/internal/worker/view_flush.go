package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type Counter interface {
	DirtyMembers(ctx context.Context) ([]uint64, error)
	DrainIncrement(ctx context.Context, id uint64) (int64, error)
	Ack(ctx context.Context, ids []uint64) error
	Restore(ctx context.Context, id uint64, n int64) error
}

type Articles interface {
	IncrementViewCount(ctx context.Context, id uint64, delta int64) error
}

type ViewFlush struct {
	counter  Counter
	articles Articles
	interval time.Duration
}

func New(counter Counter, articles Articles, interval time.Duration) *ViewFlush {
	return &ViewFlush{counter: counter, articles: articles, interval: interval}
}

func (w *ViewFlush) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := w.flushOnce(final); err != nil {
				log.Error().Err(err).Msg("final view flush failed")
			}
			cancel()
			return
		case <-t.C:
			if err := w.flushOnce(ctx); err != nil {
				log.Error().Err(err).Msg("view flush failed")
			}
		}
	}
}

func (w *ViewFlush) flushOnce(ctx context.Context) error {
	ids, err := w.counter.DirtyMembers(ctx)
	if err != nil {
		return fmt.Errorf("dirty members: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	applied := make([]uint64, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		delta, err := w.counter.DrainIncrement(ctx, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if delta == 0 {
			applied = append(applied, id)
			continue
		}
		if err := w.articles.IncrementViewCount(ctx, id, delta); err != nil {
			if rErr := w.counter.Restore(ctx, id, delta); rErr != nil {
				log.Error().Err(rErr).Uint64("id", id).Msg("restore failed")
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		applied = append(applied, id)
	}
	if len(applied) > 0 {
		if err := w.counter.Ack(ctx, applied); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

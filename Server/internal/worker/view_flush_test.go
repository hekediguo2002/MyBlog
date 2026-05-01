package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubCounter struct {
	mu      sync.Mutex
	dirty   []uint64
	drained map[uint64]int64
	ack     []uint64
	restore map[uint64]int64
}

func (s *stubCounter) Inc(ctx context.Context, id uint64) error { return nil }
func (s *stubCounter) GetIncrement(ctx context.Context, id uint64) (int64, error) { return 0, nil }
func (s *stubCounter) DirtyMembers(ctx context.Context) ([]uint64, error)     { return s.dirty, nil }
func (s *stubCounter) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v := s.drained[id]
	return v, nil
}
func (s *stubCounter) Ack(ctx context.Context, ids []uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ack = append(s.ack, ids...)
	return nil
}
func (s *stubCounter) Restore(ctx context.Context, id uint64, n int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restore[id] += n
	return nil
}

type stubArticles struct {
	mu     sync.Mutex
	apply  map[uint64]int64
	failOn map[uint64]bool
}

func (s *stubArticles) IncrementViewCount(ctx context.Context, id uint64, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn[id] {
		return errors.New("boom")
	}
	s.apply[id] += delta
	return nil
}

func TestFlushOnce_AppliesIncrementsAndAcks(t *testing.T) {
	cnt := &stubCounter{dirty: []uint64{1, 2}, drained: map[uint64]int64{1: 3, 2: 7}, restore: map[uint64]int64{}}
	arts := &stubArticles{apply: map[uint64]int64{}}
	w := New(cnt, arts, 30*time.Second)
	require.NoError(t, w.flushOnce(context.Background()))
	require.Equal(t, int64(3), arts.apply[1])
	require.Equal(t, int64(7), arts.apply[2])
	require.ElementsMatch(t, []uint64{1, 2}, cnt.ack)
	require.Empty(t, cnt.restore)
}

func TestFlushOnce_RestoresOnDBFailure(t *testing.T) {
	cnt := &stubCounter{dirty: []uint64{1}, drained: map[uint64]int64{1: 5}, restore: map[uint64]int64{}}
	arts := &stubArticles{apply: map[uint64]int64{}, failOn: map[uint64]bool{1: true}}
	w := New(cnt, arts, 30*time.Second)
	err := w.flushOnce(context.Background())
	require.Error(t, err)
	require.Equal(t, int64(5), cnt.restore[1])
	require.Empty(t, cnt.ack)
}

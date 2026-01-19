package workers

import (
	"context"
	"sync"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/postgres"
)

type Batcher struct {
	mu       sync.Mutex
	buffer   map[string]int
	db       *postgres.DB
	maxSize  int
	interval time.Duration
}

func (b *Batcher) Add(userID string, rating int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer[userID] = rating
}

func (b *Batcher) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush(ctx)
		case <-ctx.Done():
			b.flush(ctx)
			return
		}
	}
}

func (b *Batcher) flush(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buffer) == 0 {
		return
	}

	tx, err := b.db.Pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	for userID, rating := range b.buffer {
		_, err := tx.Exec(
			ctx,
			`UPDATE users SET rating = $1 WHERE id = $2`,
			rating,
			userID,
		)
		if err != nil {
			return
		}
	}

	tx.Commit(ctx)
	b.buffer = make(map[string]int)
}

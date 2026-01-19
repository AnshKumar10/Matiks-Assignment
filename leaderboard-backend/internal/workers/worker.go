package workers

import (
	"context"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/leaderboard"
	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/postgres"
)

type Worker struct {
	queue       Queue
	leaderboard *leaderboard.Repository
	db          *postgres.DB
	batcher     *Batcher
}

func NewWorker(
	queue Queue,
	lb *leaderboard.Repository,
	db *postgres.DB,
) *Worker {
	return &Worker{
		queue:       queue,
		leaderboard: lb,
		db:          db,
		batcher: &Batcher{
			buffer:   make(map[string]int),
			db:       db,
			maxSize:  500,
			interval: 2 * time.Second,
		},
	}
}

func (w *Worker) Run(ctx context.Context) error {
	events, err := w.queue.Consume(ctx)
	if err != nil {
		return err
	}

	go w.batcher.Run(ctx)

	for event := range events {
		w.processEvent(ctx, event)
	}

	return nil
}

func (w *Worker) processEvent(ctx context.Context, event ScoreUpdateEvent) {
	oldRating, err := w.leaderboard.GetUserRating(ctx, event.UserID)
	if err != nil {
		return
	}

	newRating := oldRating + event.Delta
	if newRating < 100 {
		newRating = 100
	}
	if newRating > 5000 {
		newRating = 5000
	}

	err = w.leaderboard.UpdateUserRating(
		ctx,
		event.UserID,
		newRating,
	)
	if err != nil {
		return
	}

	// Batched DB persistence to keep updates non-blocking (no unbounded goroutines).
	w.batcher.Add(event.UserID, newRating)
}

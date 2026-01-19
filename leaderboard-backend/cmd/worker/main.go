package main

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/config"
	"github.com/AnshKumar10/Matiks-Assignment/internal/leaderboard"
	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/postgres"
	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/redis"
	"github.com/AnshKumar10/Matiks-Assignment/internal/workers"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pg, err := postgres.New(cfg.PostgresDSN)
	if err != nil {
		log.Fatal("postgres connection failed", zap.Error(err))
	}

	redisClient := redis.New(cfg.RedisAddr, cfg.RedisPassword)
	db, _ := postgres.New(cfg.PostgresDSN)

	queue := workers.NewRedisQueue(redisClient.RDB)
	lbRepo := leaderboard.NewRepository(pg, redisClient.RDB)

	worker := workers.NewWorker(queue, lbRepo, db)
	go simulateRandomScoreUpdates(ctx, queue, db)

	if err := worker.Run(ctx); err != nil {
		log.Fatal("worker run failed", zap.Error(err))
	}
}

// simulateRandomScoreUpdates publishes random score update events every 10 seconds.
// Ratings are clamped to [100, 5000] in the worker, so we just emit random deltas here.
func simulateRandomScoreUpdates(ctx context.Context, queue workers.Queue, db *postgres.DB) {
	// Load all user IDs once for simulation.
	rows, err := db.Pool.Query(ctx, "SELECT id FROM users")
	if err != nil {
		log.Println("failed to load user ids for simulation:", err)
		return
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}

	if len(userIDs) == 0 {
		log.Println("no users found for score simulation")
		return
	}

	log.Printf("starting random score simulation for %d users\n", len(userIDs))

	rand.Seed(time.Now().UnixNano())
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	const eventsPerTick = 1000 // adjust as needed to simulate higher/lower load

	for {
		select {
		case <-ctx.Done():
			log.Println("stopping random score simulation")
			return
		case <-ticker.C:
			for i := 0; i < eventsPerTick && i < len(userIDs); i++ {
				userID := userIDs[rand.Intn(len(userIDs))]

				// Random delta between -200 and +200
				delta := rand.Intn(401) - 200

				event := workers.ScoreUpdateEvent{
					UserID:    userID,
					Delta:     delta,
					Timestamp: time.Now(),
				}

				// Fire-and-forget; if it fails occasionally it's fine for simulation.
				if err := queue.Publish(ctx, event); err != nil {
					log.Println("failed to publish score update event:", err)
				}
			}
		}
	}
}

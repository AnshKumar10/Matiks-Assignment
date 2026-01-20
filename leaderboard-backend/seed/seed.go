package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	LeaderboardKey      = "leaderboard:global"
	LeaderboardRatings  = "leaderboard:ratings"
	LeaderboardCounters = "leaderboard:rating_counts"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()

	// Number of users
	numUsers := 10000
	if v := os.Getenv("SEED_USERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			numUsers = n
		}
	}

	// Postgres
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:@localhost:5432/leaderboard?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot reach Postgres:", err)
	}

	// Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	// Schema
	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		rating INT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_users_username_pattern
	ON users (username text_pattern_ops);
	`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatal("Failed to create table:", err)
	}

	log.Println("Schema ready.")

	// Cleanup old data
	log.Println("Deleting existing users...")
	if _, err := db.Exec(`DELETE FROM users`); err != nil {
		log.Fatal("Failed to delete users:", err)
	}

	log.Println("Clearing Redis leaderboard...")
	rdb.Del(ctx, LeaderboardKey, LeaderboardRatings, LeaderboardCounters)

	// Prepare insert
	stmt, err := db.Prepare(`
		INSERT INTO users (id, username, rating, created_at)
		VALUES ($1,$2,$3,$4)
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	popularRatings := []int{1500, 2000, 2500, 3000, 3500, 4000, 4500}

	log.Printf("Seeding %d users...\n", numUsers)

	inserted := 0
	for inserted < numUsers {
		id := uuid.New().String()
		username := generateUsername()

		var rating int
		if rand.Float64() < 0.2 {
			rating = popularRatings[rand.Intn(len(popularRatings))]
		} else {
			rating = rand.Intn(4901) + 100
		}

		createdAt := time.Now().Add(-time.Duration(rand.Intn(720)) * time.Hour)

		_, err := stmt.Exec(id, username, rating, createdAt)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			log.Fatal("Insert failed:", err)
		}

		inserted++

		if inserted%1000 == 0 {
			log.Printf("%d users inserted...", inserted)
		}
	}

	log.Println("Users seeded. Rebuilding Redis leaderboard...")

	rows, err := db.Query(`SELECT id, rating FROM users`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	pipe := rdb.Pipeline()
	count := 0

	for rows.Next() {
		var id string
		var rating int
		rows.Scan(&id, &rating)

		pipe.ZAdd(ctx, LeaderboardKey, redis.Z{
			Score:  float64(rating),
			Member: id,
		})

		pipe.ZAdd(ctx, LeaderboardRatings, redis.Z{
			Score:  float64(rating),
			Member: fmt.Sprintf("%d", rating),
		})

		pipe.HIncrBy(ctx, LeaderboardCounters, fmt.Sprintf("%d", rating), 1)

		count++

		if count%2000 == 0 {
			pipe.Exec(ctx)
			pipe = rdb.Pipeline()
		}
	}

	pipe.Exec(ctx)

	log.Printf("Redis rebuild complete. %d users indexed.\n", count)
	log.Println("Seeding finished successfully.")
}

func generateUsername() string {
	firstNames := []string{
		"alex", "sam", "jordan", "taylor", "morgan", "chris", "jamie",
		"olivia", "noah", "emma", "liam", "ava", "mia", "lucas", "milo",
		"sophia", "isabella", "ethan", "logan", "harper", "amelia", "ella",
	}

	adjectives := []string{
		"fast", "cool", "brave", "smart", "lucky", "silent", "wild",
		"happy", "fierce", "chill", "mighty", "sly", "bold", "swift",
	}

	return fmt.Sprintf(
		"%s_%s_%d",
		adjectives[rand.Intn(len(adjectives))],
		firstNames[rand.Intn(len(firstNames))],
		rand.Intn(1_000_000_000),
	)
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (stringIndex(s, substr) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

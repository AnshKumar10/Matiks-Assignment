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

func main() {
	rand.Seed(time.Now().UnixNano())

	numUsers := 10000
	if v := os.Getenv("SEED_USERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			numUsers = n
		}
	}

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@postgres:5432/leaderboard?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot reach Postgres:", err)
	}

	log.Println("Seeding database with users...")

	stmt, _ := db.Prepare(`
		INSERT INTO users (id, username, rating, created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (id) DO NOTHING
	`)
	defer stmt.Close()

	popularRatings := []int{1500, 2000, 2500, 3000, 3500, 4000, 4500}

	usedUsernames := make(map[string]bool)

	for i := 0; i < numUsers; i++ {
		id := uuid.New().String()
		username := uniqueUsername(usedUsernames)

		var rating int
		if rand.Float64() < 0.2 {
			rating = popularRatings[rand.Intn(len(popularRatings))]
		} else {
			rating = rand.Intn(4901) + 100
		}

		createdAt := time.Now().Add(-time.Duration(rand.Intn(1000*24)) * time.Hour)

		_, err := stmt.Exec(id, username, rating, createdAt)
		if err != nil {
			log.Println("Error inserting:", err)
		}

		if (i+1)%1000 == 0 {
			log.Printf("%d users inserted...", i+1)
		}
	}

	log.Println("Seeding complete. Rebuilding Redis leaderboard...")

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})
	defer rdb.Close()
	ctx := context.Background()
	rdb.Del(ctx, "leaderboard:global", "leaderboard:ratings", "leaderboard:rating_counts")

	rows, _ := db.Query("SELECT id, rating FROM users")
	defer rows.Close()
	for rows.Next() {
		var id string
		var rating int
		rows.Scan(&id, &rating)

		rdb.ZAdd(ctx, "leaderboard:global", redis.Z{Score: float64(rating), Member: id})
		rdb.ZAdd(ctx, "leaderboard:ratings", redis.Z{Score: float64(rating), Member: fmt.Sprintf("%d", rating)})
		rdb.HIncrBy(ctx, "leaderboard:rating_counts", fmt.Sprintf("%d", rating), 1)
	}

	log.Println("Redis leaderboard rebuilt successfully.")
}

func uniqueUsername(used map[string]bool) string {
	for {
		u := randomUsername()
		if !used[u] {
			used[u] = true
			return u
		}
	}
}

func randomUsername() string {
	firstNames := []string{
		"alex", "sam", "jordan", "taylor", "morgan", "chris", "jamie",
		"olivia", "noah", "emma", "liam", "ava", "mia", "lucas", "milo",
		"sophia", "isabella", "ethan", "logan", "harper", "amelia", "ella",
		"jack", "leo", "lily", "hannah", "sebastian", "nora", "finn", "ariel",
		"ryan", "zoe", "carter", "audrey", "owen", "elijah", "claire", "ivy",
		"jackson", "grace", "dylan", "scarlett", "aiden", "lucia", "luna",
		"ryder", "eli", "caden", "stella", "sawyer", "naomi", "nathan", "bella",
		"emma", "mason", "eva", "hunter", "leah", "joseph", "madeline", "alexander",
		"amelie", "james", "sophia", "sebby", "harley", "freya", "kai", "violet",
		"tobias", "clara", "simon", "lena", "felix", "ivy", "hugo", "maya",
		"lucien", "ella", "emmett", "amelia", "theo", "zoey", "matthew", "sienna",
		"ryland", "juliet", "nolan", "ariana", "asher", "lila", "grayson", "phoebe",
		"caius", "nina", "evan", "dahlia", "brandon", "celeste", "jude", "marie",
	}

	adjectives := []string{
		"fast", "cool", "brave", "smart", "lucky", "silent", "wild",
		"happy", "fierce", "chill", "mighty", "sly", "bold", "swift",
		"gentle", "sharp", "keen", "fiery", "graceful", "mystic", "bright",
		"noble", "calm", "stormy", "witty", "vivid", "strong", "quick",
		"proud", "steady", "crazy", "lively", "savage", "glorious", "daring",
		"clever", "brilliant", "radiant", "fearless", "spirited", "vigilant",
		"merry", "stubborn", "humble", "cheerful", "fabled", "legendary", "majestic",
		"radiant", "fiery", "gritty", "tenacious", "dashing", "shiny", "swift-footed",
		"stealthy", "valiant", "dauntless", "agile", "keen-eyed", "resolute", "bold-hearted",
		"noble-minded", "fierce-hearted", "prudent", "wily", "sneaky", "fearsome", "glimmering",
	}

	name := firstNames[rand.Intn(len(firstNames))]
	adj := adjectives[rand.Intn(len(adjectives))]
	number := rand.Intn(10000)

	return fmt.Sprintf("%s_%s%d", adj, name, number)
}

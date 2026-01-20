package leaderboard

import (
	"context"
	"fmt"
	"sort"

	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/postgres"
	"github.com/redis/go-redis/v9"
)

var updateUserRatingScript = redis.NewScript(`
-- KEYS[1] = leaderboard:global (sorted set of users)
-- KEYS[2] = leaderboard:ratings (sorted set of distinct ratings; member is rating string)
-- KEYS[3] = leaderboard:rating_counts (hash rating->count)
-- ARGV[1] = user_id
-- ARGV[2] = new_rating (string/int)

local user_id = ARGV[1]
local new_rating = tostring(ARGV[2])

local old_score = redis.call("ZSCORE", KEYS[1], user_id)

redis.call("ZADD", KEYS[1], new_rating, user_id)

-- increment new rating count + ensure rating exists in distinct set
local new_count = redis.call("HINCRBY", KEYS[3], new_rating, 1)
redis.call("ZADD", KEYS[2], new_rating, new_rating)

-- decrement old rating count and cleanup if needed
if old_score then
  local old_rating = tostring(math.floor(tonumber(old_score)))
  if old_rating ~= new_rating then
    local old_count = redis.call("HINCRBY", KEYS[3], old_rating, -1)
    if old_count <= 0 then
      redis.call("HDEL", KEYS[3], old_rating)
      redis.call("ZREM", KEYS[2], old_rating)
    end
  end
end

return new_count
`)

type Repository struct {
	db  *postgres.DB
	rdb *redis.Client
}

type LeaderboardEntry struct {
	UserID   string
	Username string
	Rating   int
	Rank     int
}

func NewRepository(db *postgres.DB, rdb *redis.Client) *Repository {
	return &Repository{
		db:  db,
		rdb: rdb,
	}
}

func (r *Repository) GetUserRating(ctx context.Context, userID string) (int, error) {
	score, err := r.rdb.ZScore(ctx, GlobalLeaderboardKey, userID).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int(score), nil
}

func (r *Repository) UpdateUserRating(ctx context.Context, userID string, newRating int) error {
	// Update leaderboard + dense-ranking metadata atomically.
	_, err := updateUserRatingScript.Run(ctx, r.rdb,
		[]string{GlobalLeaderboardKey, RatingsSetKey, RatingCountsKey},
		userID,
		newRating,
	).Result()
	return err
}

func (r *Repository) GetDenseRank(ctx context.Context, userID string) (int, error) {
	rating, err := r.GetUserRating(ctx, userID)
	if err != nil {
		return 0, err
	}
	if rating == 0 {
		return 0, nil
	}

	// Dense rank = 1 + number of DISTINCT ratings strictly greater than user's rating.
	min := fmt.Sprintf("(%d", rating) // '(' makes it exclusive: (rating, +inf]
	count, err := r.rdb.ZCount(ctx, RatingsSetKey, min, "+inf").Result()
	if err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

func (r *Repository) GetTop(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	zs, err := r.rdb.ZRevRangeWithScores(ctx, GlobalLeaderboardKey, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]LeaderboardEntry, len(zs))
	for i, z := range zs {
		entries[i] = LeaderboardEntry{
			UserID: z.Member.(string),
			Rating: int(z.Score),
		}
	}

	// Fill dense ranks based on ratings set.
	if len(entries) == 0 {
		return entries, nil
	}

	ranks, err := r.GetDenseRanks(ctx, entries)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if rank, ok := ranks[entries[i].UserID]; ok {
			entries[i].Rank = rank
		}
	}

	return entries, nil
}

func (r *Repository) GetNearby(ctx context.Context, userID string, radius int) ([]LeaderboardEntry, error) {
	rank, err := r.rdb.ZRevRank(ctx, GlobalLeaderboardKey, userID).Result()
	if err != nil {
		return nil, err
	}

	start := int(rank) - radius
	if start < 0 {
		start = 0
	}
	stop := int(rank) + radius

	zs, err := r.rdb.ZRevRangeWithScores(ctx, GlobalLeaderboardKey, int64(start), int64(stop)).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]LeaderboardEntry, len(zs))
	for i, z := range zs {
		entries[i] = LeaderboardEntry{
			UserID: z.Member.(string),
			Rating: int(z.Score),
		}
	}
	if len(entries) == 0 {
		return entries, nil
	}

	ranks, err := r.GetDenseRanks(ctx, entries)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if rank, ok := ranks[entries[i].UserID]; ok {
			entries[i].Rank = rank
		}
	}

	return entries, nil
}

// GetUsername returns a single username
func (r *Repository) GetUsername(ctx context.Context, userID string) (string, error) {
	var username string
	err := r.db.Pool.QueryRow(ctx, "SELECT username FROM users WHERE id=$1", userID).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}

func (r *Repository) GetUsernames(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}

	query := `SELECT id, username FROM users WHERE id = ANY($1)`
	rows, err := r.db.Pool.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usernames := make(map[string]string)
	for rows.Next() {
		var id, username string
		if err := rows.Scan(&id, &username); err != nil {
			return nil, err
		}
		usernames[id] = username
	}
	return usernames, nil
}

func (r *Repository) SearchUsers(ctx context.Context, query string) ([]LeaderboardEntry, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, username FROM users 
         WHERE username ILIKE $1
         LIMIT 50`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.Username); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	// Fill LIVE ratings + dense ranks from Redis (avoid stale Postgres rating).
	if len(entries) == 0 {
		return entries, nil
	}

	userIDs := make([]string, 0, len(entries))
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}

	scores, err := r.rdb.ZMScore(ctx, GlobalLeaderboardKey, userIDs...).Result()
	if err != nil {
		return nil, err
	}
	for i := range entries {
		// ZMScore returns 0 for missing members; treat as "not ranked".
		entries[i].Rating = int(scores[i])
	}

	ranks, err := r.GetDenseRanks(ctx, entries)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if rank, ok := ranks[entries[i].UserID]; ok {
			entries[i].Rank = rank
		}
	}

	// Sort by rank (ascending: rank 1 is best)
	sort.Slice(entries, func(i, j int) bool {
		// Users without rank (rank 0) go to the end
		if entries[i].Rank == 0 {
			return false
		}
		if entries[j].Rank == 0 {
			return true
		}
		return entries[i].Rank < entries[j].Rank
	})

	return entries, nil
}

// GetDenseRanks returns dense ranks for many users in a single Redis pipeline.
// Users with the same rating get the same dense rank.
func (r *Repository) GetDenseRanks(ctx context.Context, entries []LeaderboardEntry) (map[string]int, error) {
	// Group users by rating so we compute rank once per distinct rating.
	ratingToUserIDs := make(map[int][]string)
	for _, e := range entries {
		if e.Rating == 0 {
			continue
		}
		ratingToUserIDs[e.Rating] = append(ratingToUserIDs[e.Rating], e.UserID)
	}

	if len(ratingToUserIDs) == 0 {
		return map[string]int{}, nil
	}

	pipe := r.rdb.Pipeline()
	ratingCmds := make(map[int]*redis.IntCmd, len(ratingToUserIDs))

	for rating := range ratingToUserIDs {
		min := fmt.Sprintf("(%d", rating)
		ratingCmds[rating] = pipe.ZCount(ctx, RatingsSetKey, min, "+inf")
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	out := make(map[string]int, len(entries))
	for rating, userIDs := range ratingToUserIDs {
		cmd := ratingCmds[rating]
		count, err := cmd.Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		rank := int(count) + 1
		for _, id := range userIDs {
			out[id] = rank
		}
	}

	return out, nil
}

func (r *Repository) GetAllUserIDs(ctx context.Context) ([]string, error) {

	
    userIDs, err := r.rdb.ZRange(ctx, GlobalLeaderboardKey, 0, -1).Result()
    if err != nil {
        return nil, err
    }
    return userIDs, nil
}
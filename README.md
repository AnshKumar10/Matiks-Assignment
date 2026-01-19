## Matiks Leaderboard – Implementation & Flow

This project is a full leaderboard system designed to:

- **Manage 10,000+ users now, and scale to millions**
- **Calculate correct dense ranks** (same rating ⇒ same rank, no gaps)
- **Simulate continuous random score updates**
- **Allow live search by username and return global rank + rating**

The repo has two main parts:

- `leaderboard-backend/` – Go HTTP API, workers, Redis + Postgres integration
- `leaderboard-ui/` – React Native (Expo) app that visualizes the leaderboard

---

### 1. Data Model & Storage

- **PostgreSQL** (source of truth)
  - Table: `users`
  - Columns: `id (UUID)`, `username (TEXT UNIQUE)`, `rating (INT)`, `created_at (TIMESTAMP)`
  - Indexed for search on `username` using `text_pattern_ops`.

- **Redis** (real-time ranking engine)
  - `leaderboard:global` – **Sorted Set**
    - member: `user_id`
    - score: `rating`
  - `leaderboard:ratings` – **Sorted Set of distinct ratings**
    - member: `rating` as string (e.g. `"3900"`)
    - score: `rating` as number
  - `leaderboard:rating_counts` – **Hash**
    - key: `rating` as string
    - value: number of users currently holding that rating

Ratings are always between **100 and 5000** (clamped everywhere ratings are updated).

---

### 2. Seeding & Initial Load (10,000+ Users)

File: `leaderboard-backend/seed/seed.go`

- Creates **10,000 users by default** (override with `SEED_USERS`).
- Inserts each user into Postgres with:
  - Unique `username`
  - `rating` in `[100, 5000]`
  - Skew toward some **popular rating buckets** to create realistic ties.
- After inserting users, it **rebuilds Redis**:
  - Clears `leaderboard:global`, `leaderboard:ratings`, `leaderboard:rating_counts`.
  - Scans all `id, rating` pairs from `users`.
  - For each row:
    - `ZADD leaderboard:global rating user_id`
    - `ZADD leaderboard:ratings rating "rating"`
    - `HINCRBY leaderboard:rating_counts "rating" 1`

Result: Postgres is the persistent source of truth, and Redis is primed with a consistent leaderboard and dense-ranking metadata.

---

### 3. Ranking Logic (Dense Ranking)

**Goal:** Users with the same rating share the same rank; ranks are compact (no gaps).

Definition:

- For a user with rating \(R\):
  - `rank = 1 + number of DISTINCT ratings strictly greater than R`

Implementation (file `internal/leaderboard/repository.go`):

- `GetUserRating(ctx, userID)` – reads the user’s rating from `leaderboard:global` via `ZSCORE`.
- `GetDenseRank(ctx, userID)`:
  - Calls `GetUserRating`.
  - Uses `ZCOUNT leaderboard:ratings (R +inf` to count **distinct** ratings > R.
  - Returns `count + 1` as the dense rank.

- **Batch dense rank**:
  - `GetDenseRanks(ctx, entries []LeaderboardEntry)`:
    - Groups users by rating.
    - For each distinct rating:
      - Issues a `ZCOUNT leaderboard:ratings (rating +inf` using a Redis pipeline.
    - Assigns the same computed rank to all users with that rating.
    - This is used by:
      - `GetTop` (top leaderboard)
      - `GetNearby` (±N window)
      - `SearchUsers` (search results)

This guarantees that **ties share ranks**, and ranking stays correct even as scores change.

---

### 4. Score Updates & Background Worker

#### 4.1. Score Update Events

Type: `internal/workers/ScoreUpdateEvent`

- Fields:
  - `user_id`
  - `delta` (rating change)
  - `timestamp`

Events are stored in Redis Streams (`score_updates`), handled by `RedisQueue`:

- `Publish(ctx, event)` – serializes event as JSON and `XADD` into the stream.
- `Consume(ctx)` – `XREAD` in a loop and yields events via a Go channel.

#### 4.2. Worker Process

File: `cmd/worker/main.go` and `internal/workers/worker.go`

- Sets up:
  - Postgres connection
  - Redis client
  - `leaderboard.Repository`
  - `RedisQueue` implementation of `Queue`
  - `Worker` instance
- Also starts **random score simulation** (see section 5).

The `Worker`:

- Consumes events from the queue.
- For each `ScoreUpdateEvent`:
  - Loads old rating from Redis via `GetUserRating`.
  - Computes `newRating = clamp(oldRating + delta, 100, 5000)`.
  - Calls `UpdateUserRating` on the repository (atomic Lua script; see below).
  - Enqueues the `(userID, newRating)` into a **batcher** for Postgres persistence.

#### 4.3. Atomic UpdateUserRating (Dense-Rank Metadata)

In `internal/leaderboard/repository.go`:

- `UpdateUserRating` uses a **Redis Lua script** (`updateUserRatingScript`) with keys:
  - `leaderboard:global` (sorted set)
  - `leaderboard:ratings` (distinct ratings sorted set)
  - `leaderboard:rating_counts` (hash)

Flow:

1. Get old score from `leaderboard:global` via `ZSCORE`.
2. `ZADD leaderboard:global new_rating user_id`.
3. Increment count and ensure new rating is in sets:
   - `HINCRBY leaderboard:rating_counts new_rating 1`
   - `ZADD leaderboard:ratings new_rating "new_rating"`.
4. If the user’s old rating is different:
   - `HINCRBY leaderboard:rating_counts old_rating -1`.
   - If count becomes `<= 0`, **remove** that rating from:
     - `leaderboard:rating_counts`
     - `leaderboard:ratings`.

Result:

- Distinct ratings set and counts are always correct.
- Dense ranking based on `leaderboard:ratings` stays accurate over time, even under heavy updates.

#### 4.4. Batched Postgres Persistence

File: `internal/workers/batcher.go` and `internal/workers/worker.go`

- `Batcher`:
  - Maintains an in-memory buffer `map[userID]rating`.
  - Every `interval` (2s), or on shutdown:
    - Begins a transaction.
    - Executes `UPDATE users SET rating = $1 WHERE id = $2` for all buffered entries.
    - Commits and clears the buffer.
- `Worker`:
  - Owns a `Batcher` instance.
  - Calls `batcher.Run(ctx)` in a goroutine.
  - After each rating update, calls `batcher.Add(userID, newRating)`.

This avoids spawning unbounded goroutines and makes DB writes efficient and non-blocking.

---

### 5. Random Score Simulation

File: `cmd/worker/main.go`

Function: `simulateRandomScoreUpdates(ctx, queue, db)`

- On worker startup, it:
  - Loads all `id` values from the `users` table in Postgres.
  - Every **10 seconds**:
    - Picks many random users (configurable count).
    - For each:
      - Generates a random `delta` in a range like `[-200, +200]`.
      - Publishes a `ScoreUpdateEvent` to the Redis queue.

The worker then processes these events, updating Redis and Postgres. Ratings are clamped to **[100, 5000]** so invariants are maintained.

---

### 6. HTTP API (Backend)

Router file: `internal/http/router.go`  
Handlers: `internal/http/handlers/*.go`

Base URL (example): `http://<host>:<port>`

- **Health Check**
  - `GET /health`
  - Returns `"ok"`.

- **Leaderboard**
  - `GET /leaderboard?limit=&offset=`
    - Returns top players with:
      - `user_id`
      - `username`
      - `rating`
      - `rank` (dense rank)
    - Uses Redis `ZREVRANGE` + dense rank computation from `leaderboard:ratings`.

  - `GET /leaderboard/nearby?user_id=...`
    - Returns players **around a given user** (±4 positions).
    - Uses `ZREVRANK` + `ZREVRANGE` + dense rank.

  - `GET /leaderboard/me?user_id=...`
    - Returns current user’s:
      - `user_id`
      - `username`
      - `rating`
      - `rank` (dense)

- **User Search**
  - `GET /leaderboard/users/search?q=<username-fragment>`
    - Step 1 (Postgres): find users by `username ILIKE '%q%'` (max 50).
    - Step 2 (Redis): for those `user_id`s:
      - `ZMScore leaderboard:global user_ids...` → **live rating**
      - `GetDenseRanks` → **live dense rank**.
    - Returns list of:
      - `user_id`
      - `username`
      - `rating` (current, from Redis)
      - `rank` (dense, from Redis metadata)

  This ensures that **even while thousands of updates/second happen**, search immediately reflects the latest global rank.

- **Score Update API (optional, for manual testing)**
  - `POST /score/update`
  - Body: `{ "user_id": "...", "delta": 25 }`
  - Enqueues a `ScoreUpdateEvent` for the worker to process.

---

### 7. Frontend (React Native / Expo)

Entry: `leaderboard-ui/app/index.tsx`  
Screen: `leaderboard-ui/app/leaderboard.tsx`  
API client: `leaderboard-ui/app/api.ts`

#### 7.1. Leaderboard Screen

- On mount:
  - Calls `getLeaderboard()` to fetch data from `/leaderboard`.
  - Starts an interval (e.g. every **15 seconds**) to re-fetch the leaderboard so the UI stays in sync with backend updates.
- Pull-to-refresh:
  - The leaderboard `ScrollView` has a `RefreshControl`, so the user can **swipe down to refresh** immediately.
- Display:
  - For each user:
    - `rank` – used to style top 3 differently.
    - `username`
    - `rating`

#### 7.2. User Search

- Search input at the top of the screen.
- Uses a debounced handler (`utils.debounce`) to avoid spamming requests.
- Calls `searchUsers(q)` → `/leaderboard/users/search?q=...`.
- If search query is non-empty, the screen shows **search results** instead of the global top N.
- For each result:
  - `rank` – **global dense rank**
  - `username`
  - `rating` – live rating from Redis

UX goals:

- Search feels instant (Postgres index + batched Redis lookups).
- UI remains responsive (debouncing + background updates).

---

### 8. Running the System

From the project root:

1. **Backend**
   - Ensure Postgres + Redis are running (or use provided Docker setup in `leaderboard-backend`).
   - Seed data:
     - `cd leaderboard-backend/seed`
     - `go run .` (set `POSTGRES_DSN`, `REDIS_ADDR` if needed).
   - Start API:
     - `cd leaderboard-backend`
     - `go run ./cmd/api`
   - Start worker:
     - `cd leaderboard-backend`
     - `go run ./cmd/worker`

2. **Frontend**
   - `cd leaderboard-ui`
   - Install deps: `npm install`
   - Start Expo dev server: `npm run start`
   - Make sure `API_BASE` in `app/api.ts` points to your backend host/port.

---

### 9. How This Meets the Requirements

- **Scales to millions of users**
  - Redis Sorted Sets + dense-ranking metadata are \(O(\log N)\) per operation.
  - API servers are stateless and horizontally scalable.

- **Correct ranking with ties**
  - Dense ranking via `leaderboard:ratings` and `ZCOUNT`.
  - Distinct rating set is **kept consistent** by the Lua script + `rating_counts` hash.

- **Random score updates**
  - Continuous simulation in the worker.
  - Real clients could send events through the same queue interface.

- **Search returns live global rank + rating**
  - Usernames from Postgres, rating + rank from Redis at query time.

- **Performance and UX**
  - Background worker + batched DB writes keep updates off the hot path.
  - Leaderboard screen auto-refresh + pull-to-refresh give a “live” feel.
  - Debounced search and light responses keep the app smooth even with 10k+ users.


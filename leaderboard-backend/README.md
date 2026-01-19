# Gaming Leaderboard System – Detailed Design Report

## 1. Overview

This document describes the design of a real-time gaming leaderboard system capable of:

- Managing 10,000+ concurrent users and scaling to millions of users
- Handling 2,500+ score updates per second
- Providing instant rank lookup
- Supporting tie-aware **dense ranking** (same rating → same rank, next rank increments by 1)
- Showing:
  - Top 10 players
  - Any user’s global rank
  - 4 players above and below a given user
- Delivering real-time updates to a React Native (Expo) frontend

Backend is implemented using Golang.

---

## 2. Functional Requirements

- Display top 10 leaderboard
- Display user rank by username (including partial search)
- Display nearby players (±4 positions)
- Handle rating range: 100–5000
- Users with same rating must share same rank (dense ranking)
- Support real-time score updates

---

## 3. Non-Functional Requirements

- Search latency < 50ms
- Leaderboard fetch latency < 100ms
- Support 5 million DAU
- Support 2.5k score updates/second
- High availability
- Horizontal scalability
- Fault tolerance
- Eventual consistency allowed (few ms)

---

## 4. Traffic Estimation

| Metric | Estimate |
|--------|----------|
| DAU | 5,000,000 |
| Concurrent users | ~150,000 |
| Score updates | 2,500/sec |
| Rank lookups | 10,000+/sec |
| Leaderboard fetch | 2,000/sec |

---

## 5. Final Architecture

React Native App  
→ API Gateway  
→ Go Backend (Stateless)  
→ Redis Cluster (Sorted Sets) + PostgreSQL  
→ Background Workers (Score updates)

---

## 6. Data Model

### PostgreSQL (Source of Truth)

```sql
users (
  id UUID PRIMARY KEY,
  username TEXT UNIQUE,
  rating INT,
  created_at TIMESTAMP
)
```

Indexes:

```sql
CREATE INDEX idx_users_username_pattern
ON users (username text_pattern_ops);
```

Used for:

- Username search
- User metadata
- Recovery and rebuild

---

### Redis (Ranking Engine)

#### 1. Global leaderboard (users)

- Key: leaderboard:global
- Type: Sorted Set
- Member: user_id
- Score: rating

Example:

```
ZADD leaderboard:global 2450 user_123
```

#### 2. Ratings set (for dense ranking)

- Key: leaderboard:ratings
- Type: Sorted Set
- Member: rating (string)
- Score: rating (number)

Example:

```
ZADD leaderboard:ratings 3900 "3900"
```

This set contains **unique rating values only**.

---

## 7. Ranking Logic (Dense Ranking – Tie Handling)

### Definition

A user’s rank is:

```
1 + number of DISTINCT ratings strictly greater than the user’s rating
```

This produces:

- Same rating → same rank
- No gaps between ranks

Example:

| User | Rating | Rank |
|------|--------|------|
| A | 5000 | 1 |
| B | 5000 | 1 |
| C | 4800 | 2 |
| D | 3900 | 3 |
| E | 3900 | 3 |
| F | 1200 | 4 |

---

### Redis Command for Rank

Given rating = R:

```
rank = ZCOUNT leaderboard:ratings (R +inf + 1
```

Explanation:

- leaderboard:ratings stores unique ratings
- ZCOUNT counts how many ratings are strictly greater than R
- +1 converts to 1-based rank

---

### Rank Lookup Algorithm

1. Get rating:

```
ZSCORE leaderboard:global user_id
```

2. Compute rank:

```
ZCOUNT leaderboard:ratings (rating +inf
rank = result + 1
```

Time complexity:

- ZSCORE → O(1)
- ZCOUNT → O(log N)

---

### Batch Rank Optimization

When returning multiple users:

- Group users by rating
- Compute rank once per unique rating
- Cache per request

---

## 8. API Design

### Get Leaderboard

```
GET /leaderboard?limit=10&offset=0
```

Backend:

```
ZREVRANGE leaderboard:global offset offset+limit WITHSCORES
```

Then compute rank using leaderboard:ratings.

---

### Search Users by Username

```
GET /users/search?q=rahul
```

Flow:

1. PostgreSQL:

```sql
SELECT id, username
FROM users
WHERE username ILIKE 'rahul%'
LIMIT 20;
```

2. Redis:

```
ZMSCORE leaderboard:global user_ids...
```

3. Rank via leaderboard:ratings

Returns:

| Global Rank | Username | Rating |

---

### Get Nearby Players (±4)

1. Get index:

```
ZREVRANK leaderboard:global user_id
```

2. Fetch window:

```
ZREVRANGE leaderboard:global index-4 index+4 WITHSCORES
```

3. Compute rank for each unique rating.

---

## 9. Score Update Architecture

### High-Level Flow

Game Clients  
→ Event Producer  
→ Message Queue (Kafka / Redis Streams / SQS)  
→ Worker Pool (Golang)  
→ Redis  
→ PostgreSQL

---

### Update Event Structure

```json
{
  "user_id": "uuid",
  "delta": 25,
  "timestamp": 12345678
}
```

---

### Worker Responsibilities

For each event:

1. Read current rating from Redis
2. Apply delta
3. Clamp to [100, 5000]
4. Update leaderboard:global
5. Maintain leaderboard:ratings
6. Persist to PostgreSQL (batched, async)

---

### Worker Pseudocode

```go
for {
    event := queue.Pop()

    current := redis.ZScore("leaderboard:global", event.UserID)
    newRating := clamp(current + event.Delta)

    redis.ZAdd("leaderboard:global", newRating, event.UserID)

    redis.ZAdd("leaderboard:ratings", newRating, fmt.Sprintf("%d", newRating))

    if redis.ZCount("leaderboard:global", oldRating, oldRating) == 0 {
        redis.ZRem("leaderboard:ratings", fmt.Sprintf("%d", oldRating))
    }

    buffer.Add(event.UserID, newRating)

    if buffer.Size() >= 500 {
        postgres.BatchUpdate(buffer)
        buffer.Clear()
    }
}
```

---

### Redis Update Commands

```
ZADD leaderboard:global new_rating user_id
ZADD leaderboard:ratings new_rating "new_rating"
```

Optional cleanup:

```
ZCOUNT leaderboard:global old_rating old_rating
ZREM leaderboard:ratings "old_rating"
```

---

## 10. Real-Time Updates

Options:

- WebSocket push for top 10 leaderboard
- Periodic polling (2–5 seconds)

---

## 11. Scaling Strategy

### Backend

- Stateless Go servers
- Horizontal scaling
- Load balancer

### Redis

- Redis Cluster
- Sharding
- Replication
- AOF persistence

### PostgreSQL

- Read replicas
- Indexed username
- Batched writes

---

## 12. Failure Recovery

| Failure | Strategy |
|--------|----------|
| Redis crash | Rebuild leaderboard from PostgreSQL |
| Worker failure | Queue retry |
| DB failure | Continue Redis, replay later |
| Partial data loss | Nightly rebuild job |

---

## 13. Why Redis Sorted Sets?

| Feature | Benefit |
|--------|----------|
| In-memory | Ultra-fast |
| Sorted Sets | Natural leaderboard |
| Extra ratings set | Dense ranking |
| Atomic ops | No race conditions |
| O(log N) ops | Scales to millions |

---

## 14. Comparison With Alternatives

### A. SQL-only Leaderboard

- Slow ranking
- Expensive updates
- Poor concurrency
- Hard tie handling

Verdict: Not suitable

---

### B. NoSQL (DynamoDB / Cassandra)

- No sorted ranking
- Inefficient range queries
- Complex logic

Verdict: Poor fit

---

### C. Precomputed Rank Table

- Heavy recomputation
- Locking
- Not real-time

Verdict: Not suitable

---

### D. Redis Sorted Sets + SQL (Chosen)

| Criteria | Result |
|---------|--------|
| Performance | Excellent |
| Real-time | Yes |
| Dense ranking | Yes |
| Scalable | Yes |
| Reliability | High |

---

## 15. Trade-offs

### Pros

- Real-time ranking
- Accurate dense ranking
- Fast search
- Proven architecture
- High write throughput

### Cons

- Redis memory cost
- Cluster complexity
- Dual writes

Mitigation:

- Redis cluster
- Async DB writes
- Periodic reconciliation

---

## 16. Capacity Planning (Redis)

| Users | Memory |
|-------|--------|
| 1M | ~80MB |
| 10M | ~800MB |
| 50M | ~4GB |

---

## 17. Conclusion

This design provides:

- Real-time leaderboard
- Dense ranking correctness
- Instant username search
- Horizontal scalability
- Fault tolerance

It fully satisfies the functional and non-functional requirements for a production-grade gaming leaderboard system.

---

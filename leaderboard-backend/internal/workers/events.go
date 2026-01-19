package workers

import "time"

type ScoreUpdateEvent struct {
	UserID    string    `json:"user_id"`
	Delta     int       `json:"delta"`
	Timestamp time.Time `json:"timestamp"`
}

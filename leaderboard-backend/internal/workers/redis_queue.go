package workers

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const scoreStream = "score_updates"

type RedisQueue struct {
	rdb *redis.Client
}

func NewRedisQueue(rdb *redis.Client) *RedisQueue {
	return &RedisQueue{rdb: rdb}
}

func (q *RedisQueue) Publish(ctx context.Context, event ScoreUpdateEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: scoreStream,
		Values: map[string]interface{}{
			"data": data,
		},
	}).Err()
}

func (q *RedisQueue) Consume(ctx context.Context) (<-chan ScoreUpdateEvent, error) {
	ch := make(chan ScoreUpdateEvent)

	go func() {
		defer close(ch)

		lastID := "0"
		for {
			streams, err := q.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{scoreStream, lastID},
				Block:   0,
				Count:   10,
			}).Result()

			if err != nil {
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					lastID = msg.ID

					raw := msg.Values["data"].(string)
					var event ScoreUpdateEvent
					if err := json.Unmarshal([]byte(raw), &event); err != nil {
						continue
					}

					ch <- event
				}
			}
		}
	}()

	return ch, nil
}

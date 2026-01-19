package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	RDB *redis.Client
}

func New(addr, password string) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     100,
		MinIdleConns: 10,
	})

	return &Client{RDB: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.RDB.Ping(ctx).Err()
}

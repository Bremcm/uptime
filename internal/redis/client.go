package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

func New(ctx context.Context, addr string) (*Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr: addr,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Lua script //
var incrScript = goredis.NewScript(`
	local n = redis.call('INCR', KEYS[1])
	if n == 1 then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	return n
`)

// ========== //

func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := incrScript.Run(ctx, c.client, []string{key}, int(ttl.Seconds())).Int64()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (c *Client) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := c.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (string, bool, error) {
	token := uuid.NewString()
	ok, err := c.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

// Lua script //

var unlockScript = goredis.NewScript(`
	if redis.call('GET', KEYS[1]) == ARGV[1] then
		return redis.call('DEL', KEYS[1])
	else
		return 0
	end
`)

// ============= //

func (c *Client) Unlock(ctx context.Context, key, token string) error {
	return unlockScript.Run(ctx, c.client, []string{key}, token).Err()
}

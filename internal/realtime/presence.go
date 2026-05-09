package realtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Presence interface {
	Register(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) (bool, error)
	Unregister(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) (bool, error)
	Refresh(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) error
	OnlineUsers(ctx context.Context) ([]uuid.UUID, error)
}

type RedisPresence struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisPresence(client *redis.Client) *RedisPresence {
	return &RedisPresence{client: client, ttl: 90 * time.Second}
}

func (p *RedisPresence) Register(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) (bool, error) {
	key := presenceKey(userID)
	before, err := p.client.SCard(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if err := p.client.SAdd(ctx, key, connectionID.String()).Err(); err != nil {
		return false, err
	}
	if err := p.client.Expire(ctx, key, p.ttl).Err(); err != nil {
		return false, err
	}
	return before == 0, nil
}

func (p *RedisPresence) Unregister(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) (bool, error) {
	key := presenceKey(userID)
	if err := p.client.SRem(ctx, key, connectionID.String()).Err(); err != nil {
		return false, err
	}
	after, err := p.client.SCard(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if after == 0 {
		_ = p.client.Del(ctx, key).Err()
		return true, nil
	}
	return false, p.client.Expire(ctx, key, p.ttl).Err()
}

func (p *RedisPresence) Refresh(ctx context.Context, userID uuid.UUID, connectionID uuid.UUID) error {
	key := presenceKey(userID)
	if err := p.client.SAdd(ctx, key, connectionID.String()).Err(); err != nil {
		return err
	}
	return p.client.Expire(ctx, key, p.ttl).Err()
}

func (p *RedisPresence) OnlineUsers(ctx context.Context) ([]uuid.UUID, error) {
	keys, err := p.client.Keys(ctx, "unilo:presence:user:*:connections").Result()
	if err != nil {
		return nil, err
	}

	userIDs := make([]uuid.UUID, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 5 {
			continue
		}
		userID, err := uuid.Parse(parts[3])
		if err != nil {
			continue
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func presenceKey(userID uuid.UUID) string {
	return fmt.Sprintf("unilo:presence:user:%s:connections", userID.String())
}

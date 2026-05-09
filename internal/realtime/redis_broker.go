package realtime

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const realtimeEventsChannel = "unilo:realtime:events"

type RedisBroker struct {
	client *redis.Client
}

func NewRedisBroker(client *redis.Client) *RedisBroker {
	return &RedisBroker{client: client}
}

func (b *RedisBroker) Publish(ctx context.Context, env Envelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, realtimeEventsChannel, payload).Err()
}

func (b *RedisBroker) Subscribe(ctx context.Context, handle func(Envelope)) error {
	pubsub := b.client.Subscribe(ctx, realtimeEventsChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var env Envelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				continue
			}
			handle(env)
		}
	}
}

func (b *RedisBroker) Close() error {
	return nil
}

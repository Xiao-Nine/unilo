package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	Event        string          `json:"event"`
	RequestID    string          `json:"request_id,omitempty"`
	Data         json.RawMessage `json:"data"`
	TargetUserID *uuid.UUID      `json:"target_user_id,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Broker interface {
	Publish(ctx context.Context, env Envelope) error
	Subscribe(ctx context.Context, handle func(Envelope)) error
	Close() error
}

func (e Envelope) withData(data any) Envelope {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	raw, err := json.Marshal(data)
	if err != nil {
		raw = []byte(`null`)
	}
	e.Data = raw
	return e
}

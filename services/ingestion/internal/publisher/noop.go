package publisher

import (
	"context"

	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type NoOpPublisher struct{}

func NewNoopPublisher() *NoOpPublisher {
	return &NoOpPublisher{}
}

func (n *NoOpPublisher) Publish(_ context.Context, _ *gen.VehiclePing) error { return nil }

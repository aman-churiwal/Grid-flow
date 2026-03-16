package publisher

import (
	"context"

	"github.com/aman-churiwal/gridflow-optimizer/internal/optimizer"
)

type IPublisher interface {
	Publish(ctx context.Context, event optimizer.AnomalyEvent)
}

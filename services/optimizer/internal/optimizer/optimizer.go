package optimizer

import (
	"context"

	"github.com/aman-churiwal/gridflow-optimizer/internal/consumer"
	"github.com/aman-churiwal/gridflow-shared/logger"
)

type Optimizer struct {
	msgs   <-chan consumer.Message
	logger *logger.Logger
}

func NewOptimizer(msgs <-chan consumer.Message, logger *logger.Logger) *Optimizer {
	return &Optimizer{
		msgs:   msgs,
		logger: logger,
	}
}

func (o *Optimizer) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-o.msgs:
				if !ok {
					return
				}
				o.logger.Info(ctx).
					Str("vehicle_id", msg.Ping.VehicleId).
					Msg("optimizer received ping")
				msg.Ack()
			}
		}
	}()
}

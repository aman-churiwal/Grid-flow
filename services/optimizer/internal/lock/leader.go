package lock

import (
	"context"

	"github.com/aman-churiwal/gridflow-optimizer/internal/optimizer"
	"github.com/aman-churiwal/gridflow-shared/logger"
)

func LeaderLoop(ctx context.Context, e *Election, o *optimizer.Optimizer, appLogger *logger.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		appLogger.Info(ctx).
			Str("instance_id", e.InstanceID()).
			Msg("attempting to acquire leadership")

		if err := e.Campaign(ctx); err != nil {
			if ctx.Err() != nil {
				// Context cancelled
				return
			}

			appLogger.Error(ctx).Err(err).Msg("campaign error, retrying")
			continue
		}

		appLogger.Info(ctx).
			Str("instance_id", e.InstanceID()).
			Msg("became leader")

		// Creating a child context for the optimization loop
		// Cancelling child context will stop optimizer without stopping the outer loop

		leaderContext, leaderCancel := context.WithCancel(ctx)
		o.Start(leaderContext)

		select {
		case <-e.SessionDone():
			appLogger.Warn(ctx).
				Str("instance_id", e.InstanceID()).
				Msg("session expired, lost leadership")
			leaderCancel()

		case <-ctx.Done():
			appLogger.Info(ctx).
				Str("instance_id", e.InstanceID()).
				Msg("shutting down, releasing leadership")
			leaderCancel()
			return
		}
	}
}

package optimizer

import (
	"context"

	"github.com/aman-churiwal/gridflow-optimizer/internal/consumer"
	"github.com/aman-churiwal/gridflow-shared/logger"
)

type Optimizer struct {
	msgs     <-chan consumer.Message
	detector *Detector
	geoStore *GeoStore
	logger   *logger.Logger
}

func NewOptimizer(msgs <-chan consumer.Message, detector *Detector, geoStore *GeoStore, logger *logger.Logger) *Optimizer {
	return &Optimizer{
		msgs:     msgs,
		detector: detector,
		geoStore: geoStore,
		logger:   logger,
	}
}

func (o *Optimizer) Start(ctx context.Context) {
	o.geoStore.StartPruner(ctx)
	o.detector.StartSilenceDetector(ctx)
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
				if err := o.geoStore.UpsertVehicle(ctx, msg.Ping.VehicleId, msg.Ping.Lng, msg.Ping.Lat); err != nil {
					o.logger.Error(ctx).Err(err).
						Str("vehicle_id", msg.Ping.VehicleId).
						Msg("failed to upsert vehicle")
				}
				if err := o.detector.Check(ctx, &msg.Ping); err != nil {
					o.logger.Error(ctx).Err(err).
						Str("vehicle_id", msg.Ping.VehicleId).
						Msg("anomaly check failed")
				}
				nearby := o.geoStore.FindNearby(ctx, msg.Ping.VehicleId, msg.Ping.Lng, msg.Ping.Lat)
				for _, v := range nearby {
					o.logger.Info(ctx).
						Str("vehicle_id", v.VehicleID).
						Float64("lat", v.Lat).
						Float64("lng", v.Lng).
						Msg("nearby vehicle found")
				}
				msg.Ack()
			}
		}
	}()
}

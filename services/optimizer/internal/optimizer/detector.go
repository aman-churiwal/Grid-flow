package optimizer

import (
	"context"
	"math"
	"time"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type Detector struct {
	stateStore       *StateStore
	anomalyPublisher IPublisher
	logger           *logger.Logger
}

func NewDetector(anomalyPublisher IPublisher, logger *logger.Logger) *Detector {
	stateStore := &StateStore{
		VehiclesState: make(map[string]*VehicleState),
	}

	return &Detector{
		stateStore:       stateStore,
		anomalyPublisher: anomalyPublisher,
		logger:           logger,
	}
}

func degreesToRadians(degrees float64) float64 {
	return degrees * (math.Pi / 180)
}

func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMetres = 6371000

	lat1Rad := degreesToRadians(lat1)
	lng1Rad := degreesToRadians(lng1)
	lat2Rad := degreesToRadians(lat2)
	lng2Rad := degreesToRadians(lng2)

	deltaLat := lat2Rad - lat1Rad
	deltaLng := lng2Rad - lng1Rad

	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(deltaLng/2), 2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMetres * c
}

func (d *Detector) Check(ctx context.Context, ping *gen.VehiclePing) error {
	d.stateStore.mu.Lock()
	if _, exists := d.stateStore.VehiclesState[ping.VehicleId]; !exists {
		d.stateStore.VehiclesState[ping.VehicleId] = &VehicleState{}
	}
	state := d.stateStore.VehiclesState[ping.VehicleId]

	state.LastPings = append(state.LastPings, ping)
	if len(state.LastPings) > 3 {
		state.LastPings = state.LastPings[len(state.LastPings)-3:]
	}

	state.LastSeen = time.Now()

	tempPing := state.LastRoute[0]
	state.LastRoute[1] = tempPing
	state.LastRoute[0] = ping

	state.ConnectionLostFired = false

	pings := make([]*gen.VehiclePing, len(state.LastPings))
	copy(pings, state.LastPings)
	d.stateStore.mu.Unlock()

	var publishErr error
	// ZERO SPEED CHECK
	if len(pings) >= 3 && checkZeroSpeed(pings) {
		zeroSpeedEvent := AnomalyEvent{
			VehicleID:   ping.VehicleId,
			Type:        AnomalyZeroSpeed,
			Description: "Zero speed detected",
			Lat:         ping.Lat,
			Lng:         ping.Lng,
			DetectedAt:  time.Now(),
		}
		if err := d.anomalyPublisher.Publish(ctx, zeroSpeedEvent); err != nil {
			d.logger.Error(ctx).Err(err).
				Str("vehicle_id", ping.VehicleId).
				Msg("failed to publish zero speed anomaly event")
			publishErr = err
		}
	}

	// OVER SPEED CHECK
	if ping.Speed > 120 {
		overSpeedEvent := AnomalyEvent{
			VehicleID:   ping.VehicleId,
			Type:        AnomalyOverSpeed,
			Description: "Over speed detected",
			Lat:         ping.Lat,
			Lng:         ping.Lng,
			DetectedAt:  time.Now(),
		}
		if err := d.anomalyPublisher.Publish(ctx, overSpeedEvent); err != nil {
			d.logger.Error(ctx).Err(err).
				Str("vehicle_id", ping.VehicleId).
				Msg("failed to publish over speed anomaly event")
			publishErr = err
		}
	}

	// ROUTE DEVIATION CHECK
	if len(pings) >= 2 {
		deviation := haversineDistance(pings[len(pings)-1].Lat, pings[len(pings)-1].Lng, pings[len(pings)-2].Lat, pings[len(pings)-2].Lng)
		if deviation > 500 {
			routeDeviationEvent := AnomalyEvent{
				VehicleID:   ping.VehicleId,
				Type:        AnomalyRouteDeviation,
				Description: "Route deviation detected",
				Lat:         ping.Lat,
				Lng:         ping.Lng,
				DetectedAt:  time.Now(),
			}
			if err := d.anomalyPublisher.Publish(ctx, routeDeviationEvent); err != nil {
				d.logger.Error(ctx).Err(err).
					Str("vehicle_id", ping.VehicleId).
					Msg("failed to publish route deviation anomaly event")
				publishErr = err
			}
		}
	}

	return publishErr
}

func (d *Detector) StartSilenceDetector(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.stateStore.mu.RLock()
				var events []AnomalyEvent
				for vehicleID, vehicleState := range d.stateStore.VehiclesState {
					if vehicleState.LastSeen.Add(60*time.Second).Before(time.Now()) && !vehicleState.ConnectionLostFired {
						// CONNECTION LOST EVENT
						var lat, lng float64
						if vehicleState.LastRoute[0] != nil {
							lat = vehicleState.LastRoute[0].Lat
							lng = vehicleState.LastRoute[0].Lng
						}
						events = append(events, AnomalyEvent{
							VehicleID:   vehicleID,
							Type:        AnomalyConnectionLost,
							Description: "Connection lost",
							Lat:         lat,
							Lng:         lng,
							DetectedAt:  time.Now(),
						})
					}
				}
				d.stateStore.mu.RUnlock()

				for _, event := range events {
					err := d.anomalyPublisher.Publish(ctx, event)
					if err != nil {
						d.logger.Error(ctx).Err(err).
							Str("vehicle_id", event.VehicleID).
							Msg("failed to publish connection lost anomaly event")
						continue
					}
					d.stateStore.mu.Lock()
					state := d.stateStore.VehiclesState[event.VehicleID]
					if state != nil && state.LastSeen.Add(60*time.Second).Before(time.Now()) {
						state.ConnectionLostFired = true
					}
					d.stateStore.mu.Unlock()
				}
			}
		}
	}()
}

func checkZeroSpeed(pings []*gen.VehiclePing) bool {
	for _, ping := range pings {
		if ping.Speed != 0 {
			return false
		}
	}

	return false
}

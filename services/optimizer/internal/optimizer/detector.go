package optimizer

import (
	"context"
	"time"

	"github.com/aman-churiwal/gridflow-optimizer/internal/publisher"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type Detector struct {
	stateStore *StateStore
	publisher  publisher.IPublisher
	logger     *logger.Logger
}

func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {

}

func (d *Detector) Check(ctx context.Context, ping *gen.VehiclePing) {
	d.stateStore.VehiclesState[ping.VehicleId].LastPings = append(d.stateStore.VehiclesState[ping.VehicleId].LastPings, ping)

	d.stateStore.VehiclesState[ping.VehicleId].LastSeen = time.Now()

	tempPing := d.stateStore.VehiclesState[ping.VehicleId].LastRoute[0]
	d.stateStore.VehiclesState[ping.VehicleId].LastRoute[1] = tempPing
	d.stateStore.VehiclesState[ping.VehicleId].LastRoute[0] = ping
	
}

func (d *Detector) StartSilenceDetector(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
	}()
}
